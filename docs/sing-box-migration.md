# Sing-Box 1.12.12 迁移规划

## ✅ 阶段 0 · 基线与工具

- 记录当前 `go.mod` 中的 replace / build tags，并保存失败的 `go test ./...` 输出作为基准。
- 固定工具链（Go 1.24.x）并确认工作区干净，方便定位回归。
- 添加轻量脚本（如 `scripts/check-build.sh`）执行最小化 build/test，获得快速反馈。

**完成情况：**

- `go version`：`go1.24.4 linux/amd64`（详见 `go.mod` 中 `toolchain go1.24.4`）。
- 基线测试输出存档：`docs/migration-phase0-go-test.txt`。
- 快速检查脚本：`scripts/check-build.sh`（自动打印 Go 版本并运行 `go test ./...`）。

## ✅ 阶段 1 · 依赖对齐

- 审核 sing-box 升级波及的所有依赖（`sing`、`sing-dns`、`sing-quic`、`quic-go`、`ray2sing` 等）。
- 针对每个依赖决定是沿用 upstream 还是继续维护 fork（例如保留 `hiddify-sing-box`；Stage 4 决定直接移除 `ray2sing`，不再镜像）。
- 更新 `go.mod`/`go.sum` 并运行 `go mod tidy`，确认 `github.com/sagernet/quic-go/ech` 等包可解析（必要时添加 replace）。
- 确保 CI 或脚本在依赖漂移时能快速失败。

**现状审计（摘自 `go.mod` / `go list -m all`）：**

| 模块                               | 当前指向                                                                                  | 计划动作                                                                 | 备注                                                        |
| ---------------------------------- | ----------------------------------------------------------------------------------------- | ------------------------------------------------------------------------ | ----------------------------------------------------------- |
| `github.com/sagernet/sing-box`     | 版本 `v1.12.12`，但 `replace` 到 `github.com/hiddify/hiddify-sing-box v1.8.9-0.20240928…` | 最终要移除 replace，直接使用官方 1.12.12；短期内保留 fork 以便分阶段迁移 | 需要确认 fork 与 upstream 差异，制定补丁策略                |
| `github.com/hiddify/ray2sing`      | replace 到 `github.com/hiddify/ray2sing v0.0.0-20240928…`                                 | 直接移除依赖，CLI 不再内嵌 Ray/V2Ray 链接解析（仅支持 JSON / Clash）     | Phase 4 中确认“解析功能由外部工具提供”，避免旧 API 阻断升级 |
| `github.com/sagernet/quic-go`      | 依赖 `v0.52.0-sing-box-mod.3`（无 ECH 包）                                                | 需要具备 `ech` / `http3_ech`，考虑 vendor sing-box 所用版本或自建模块    | 在决定 sing-box 来源后同步调整                              |
| `github.com/sagernet/wireguard-go` | replace 到 `github.com/hiddify/wireguard-go v0.0.0-20240727…`                             | 待评估是否仍需 fork；若 upstream 能满足则移除 replace                    | 与 Warp/移动端支持耦合                                      |
| `github.com/bepass-org/warp-plus`  | replace 到 `github.com/hiddify/warp-plus v0.0.0-20240717…`                                | 功能已从核心移除，待后续完全删掉依赖                                     | warp CLI / gRPC 已退役，暂不再维护                          |
| `github.com/sagernet/sing-*` 家族  | `sing v0.7.13`、`sing-dns v0.4.6`、`sing-quic v0.5.2-…` 等                                | 与 sing-box 1.12.12 要求一致；后续若升级需逐一验证                       | 暂不需要 replace                                            |

后续在 Phase 1 中将继续细化 `quic-go`、`wireguard-go` 的去向，并在 `docs/sing-box-migration.md` 同步记录。

## ✅ 阶段 2 · 配置与 Option 模型重构

- 全面重写 `config/` 以适配新结构：
  - 删除废弃字段（`CustomOptions`、内联 `WireGuardOptions`、`TLSTricks`、`TLSFragmentOptions` 等）。
  - 更新各类 helper（Warp patch、Selector、URL Test）以使用新的 `option.*` 布局（Listable、Duration、DialerOptions）。
- 增加兼容层，映射旧配置语义到新结构。
- 为 selector、URL Test、Warp overlay 等关键路径补充单元测试，确保序列化结果正确。

**完成情况：**

- `config/config.go:129-260` 现已统一由 `setOutbounds` 组装 URL Test、Selector、Direct Fragment 等内建出口，全面使用 `option.URLTestOutboundOptions`、`option.SelectorOutboundOptions` 与 `option.DialerOptions` 等新版结构，确保 `Listable` / `Duration` 字段序列化一致。
- `config/outbound.go:14-175` 与 `config/warp.go:177-270` 负责把旧配置里的 Warp、自定义 TLS trick、Mux、Fragment 等语义映射到新版 `option.Outbound`；在 patch 阶段就补齐 detour、静态 IP、Padding 等字段，避免运行期再做破坏性修改。
- `config/parser.go:47-134` 将 JSON / V2Ray / Clash 三种解析路径输出统一送入 `patchConfig`，借助 batch worker 并发套用新版 Warp/selector 语义，同时沿用 `libbox.CheckConfig` 做 schema 校验，确保兼容层统一出口。
- 新增 `config/config_test.go:10-123` 覆盖 selector/URL Test、Warp detour、Direct Fragment dialer 行为；`go test ./config -run TestBuildConfigAddsSelectorAndURLTest` 目前仍会命中 `option.DefaultDNSRule.Server` 等未完成的 1.12 API 映射（详见 `docs/migration-phase4-go-test.txt`），后续在 Stage 5 补齐 Dialer/Rule 兼容层后再启用。

## ✅ 阶段 3 · 运行时 / Service 更新

- 将 `v2/service_manager` 迁移到新的生命周期 API（`adapter.StartStage` 分阶段启动/停止）。
- 确认 bridge / mobile / custom 入口在新结构下编译通过（bridge build tag 已处理）。
- 保证 service 注册与关闭逻辑符合多阶段启动协议。

**完成情况：**

- `v2/service_manager/hiddify.go:12-56` 现在在 `StartServices` 前强制清理旧实例，并在 `startServiceList` 中遍历 `adapter.ListStartStages`，确保每个阶段都有机会初始化/收尾；`CloseServices` 也会串行释放 pre-service 与主 service，避免残留。
- `extension/interface.go:12-106` 里的 `extensionService` 已完整实现 `adapter.Service`：提供稳定 `Type/Tag`，并在 `adapter.StartStateStart` 阶段一次性加载/缓存启用的扩展，后续阶段 no-op，从而与多阶段协议兼容。
- `go test ./v2/service_manager` 通过编译（无测试用例），验证新的生命周期管理代码可独立 build；`go test ./custom` / `./extension` 仍受配置层尚未完成的 1.12 API 兼容阻塞（同 `docs/migration-phase4-go-test.txt`），等 Stage 5 完成后再补一次矩阵编译验证。

## ✅ 阶段 4 · 解析器退役（ray2sing 移除）

经评估，维护 `ray2sing` 以对接 sing-box 1.12.12 的成本远超收益：`ray2sing` 的 `option.Outbound` 写法停留在 1.8.x，既阻断 `go test ./config`，也让 Stage 5 的多平台验证无法继续。阶段 4 改为“退役 ray2sing，收敛解析入口”，从而解耦核心代码与旧 API。

**完成情况：**

- `go.mod` / `go.sum` 中移除 `github.com/hiddify/ray2sing`，不再需要 `replace` 指向 fork 版本。
- `config/parser.go` 只保留 JSON 与 Clash 两条解析链路，并在 JSON 失败后明确提示“Ray/V2Ray 解析已停用，请自行转换”。
- 新增 `docs/migration-phase4-go-test.txt`，记录清理依赖后的最新 `go test ./config` 输出（错误已从 `ray2sing` 切换为 `option.DefaultDNSRule` 等 Stage 5 待办项），方便追踪后续回归。

**影响面：**

- CLI 不再原生解析 V2Ray/VLESS/Trojan/TUIC 链接，文档需强调请先转换为 sing-box JSON 或 Clash 配置。
- `scripts/check-build.sh` 仍会失败，但日志已不再包含 `option.TLSTricksOptions` 相关栈，便于聚焦真正的 1.12 兼容问题。
- `configOpt.UseXrayCoreWhenPossible` 现阶段只保留开关（接口兼容），后续如需回收也需同步 CLI/UI。

### 阶段 4.1 · 剔除 ray2sing 依赖

- 从 `go.mod` 的 require / replace 列表删除 `github.com/hiddify/ray2sing`，并手动清理 go.sum 中的散列。
- 保留 `GOPROXY=direct` 的调用方式，确保后续引入新依赖时不会再次拉起 `ray2sing`。
- `go list -m github.com/hiddify/ray2sing` 现在会直接报错，验证依赖图已彻底移除。

### 阶段 4.2 · Parser 收敛

- `config/parser.go:70-103` 去掉 `ray2sing.Ray2Singbox` 分支，失败后直接进入 Clash YAML 解析；日志输出“Ray/V2Ray 解析已停用，尝试使用 Clash 配置”。
- JSON 路径仍通过 `patchConfig` 统一输出 `option.Options`，保持 Stage 2 既有兼容层。
- CLI 行为：若输入 Ray 链接，Parser 会先尝试 JSON 失败，再提示用户手动转换，无 silent fallback。

### 阶段 4.3 · 回归记录

- 新建 `docs/migration-phase4-go-test.txt`，保存 `go test ./config` 的完整失败堆栈；旧的 `docs/migration-phase0-go-test.txt` 可作为对比，证明 Stage 4 已清理掉 `TLSTricksOptions` 相关报错。
- 将该文件作为 Stage 5 的“下一个阻塞点”清单（`option.DefaultDNSRule.Server`、`C.TypeCustom` 等字段尚未完成重写），确保后续改动可量化进展。

## ✅ 阶段 5 · custom 包与 extension

### ✅ 阶段 5.1 · custom / mobile 运行时适配

**阶段 5.1.1 · Option 底层 / 上下文**

- 新增 `config/unmarshal_options.go`，集中暴露 `OptionsContext`、`UnmarshalOptions`、`MarshalOptions`；所有入口（`config/config.go`、`config/parser.go`、`config/debug.go`、`config/server.go`、`cmd/cmd_config.go`、`mobile/mobile.go`、`v2/service.go` 等）统一改为 `json.NewEncoderContext`/`Options.UnmarshalJSONContext`，彻底移除了直接操作 `MarshalJSON` 的分支。
- `BuildConfigJson`/`ToJson` 现在通过 context‐aware encoder 输出，CLI 与 SDK/服务端获取的配置串完全一致；`go test ./config` 作为回归用例记录。

**阶段 5.1.2 · Outbound/Inbound 构建**

- `config/outbound.go` 以 `option.Outbound.Options` 指针重写 patch 逻辑，涵盖 TLS trick、Mux、server domain 采集。`setOutbounds`、`setInbound`、`setDns` 全部切换为填充新版 struct（`option.URLTestOutboundOptions`、`option.TunInboundOptions`、`option.DNSServerOptions`），直接生成 selector/url-test/内置 direct without map hack。
- 默认的 direct fragment、不带 detour 的 DNS 服务器等仍保留，但现在通过 `option.DialerOptions` + `RouteAction` 驱动，避免写入已经移除的 `TLSFragmentOptions`。

**阶段 5.1.3 · Rule/DNS API**

- 所有路由/ DNS 规则由 `RuleAction`、`DNSRuleAction` 驱动：`addForceDirect`、`setRoutingOptions`、`setFakeDns` 等函数现在显式构造 `option.RawDefaultRule` + `routeActionForOutbound`/`dnsRouteActionForServer`。域名/端口匹配保持不变，但出站/服务器控制全部通过 action 表达。
- DNS Server/ FakeIP 输出完全遵循 1.12 新格式：`buildDNSServer` 先构造 legacy 结构再 `Upgrade` 到 `type: udp/tcp/https/fakeip`，`dns.fakeip` 块被 type server 取代，规则通过 action 指向 `fakeip`。
- 依赖旧 `option.Duration` 的字段统一替换为 `badoption.Duration`；RuleSet 远端也同步更新。阶段回归日志 `docs/migration-phase4-go-test.txt` 中的 `option.DefaultDNSRule.Server` 报错被消除。

**阶段 5.1.4 · 多平台入口**

- CLI、mobile、`v2` 与 `custom` 全部改用 `config.UnmarshalOptions` 读取 JSON；`mobile.BuildConfig`、`v2/standalone.go`、`cmd/cmd_config.go` 等不再直接触碰 `options.UnmarshalJSON`。
- v2/command handlers 适配最新 `libbox.CommandClientHandler`（新增 `ClearLogs`/`WriteLogs`/`WriteConnections`）并切换 `CommandGroupExpand` 常量，确保 gRPC/命令客户端在新 runtime 下仍能交互。

### ✅ 阶段 5.2 · extension server & SDK

**阶段 5.2.1 · Extension 配置基线**

- 新增 `config.NormalizeHiddifyOptions`/`LoadHiddifyOptions`/`SaveHiddifyOptions`，`v2/custom.go` 在启动时自动加载 `hiddify-settings.json` 并通过 `ChangeHiddifySettings` 持久化；全局统一走 `ensureHiddifyOptions`，避免 extension / CLI 生成出不同的 Options。
- Core gRPC 暴露 `GetHiddifySettings`，extension UI (`connectionPage.js`) 启动时自动预填表单，省去手动复制默认值的步骤。
- 前端依赖统一改用 pnpm：新增 `pnpm-lock.yaml`、`packageManager` 字段，`lib_install` / `make protos` 通过 `pnpm install --frozen-lockfile` 与 `pnpm exec` 执行 `grpc_tools_node_protoc`、`browserify`，避免 `npx`/`npm install` 带来的锁冲突。
- 鉴于 Clean IP 扩展的场景已由外部工具覆盖，移除了 `github.com/hiddify/hiddify-ip-scanner-extension` 依赖，仓库默认只内置 demo 扩展；如需继续使用可按需独立拉取。

**阶段 5.2.2 · TLS/ECH/QUIC & Proto**

- `hiddify.proto` 新增 `ConfigCapabilityResponse`/`GetConfigCapabilities`，Core 侧返回 TLS Fragment/QUIC/ECH 支持以及当前 schema 版本；UI 新增“Core Capabilities” 卡片实时展示。
- 同步生成 `hiddifyrpc/*.pb.go`、`extension/html/rpc/*_pb.js` 与 `extension/html/rpc.js`：`package.json` 引入 `grpc-tools`，配合 `PATH=$PWD/node_modules/.bin:$PATH grpc_tools_node_protoc ...` 与本地的 `protoc-gen-grpc-web` 即可完成全量产物刷新。

**阶段 5.2.3 · CLI / Extension 联动**

- `cmd extension` 增加 `--base-path/--work-path/--temp-path/--grpc-addr/--web-addr/--headless`，并由 `extension/server` 的 `ServerOptions` 统一驱动 gRPC / Web Server；headless 模式可供 CI 运行。
- 新增 `scripts/check-extension.sh`，在 CI / 本地 `./cmd.sh extension --headless` 后用 `curl -k https://127.0.0.1:<port>` 冒烟校验，失败会自动 dump log。

### ✅ 阶段 5.3 · 多平台验证

**阶段 5.3.1 · 命令矩阵**

- `docs/migration-phase5-validation.md` 现已记录 Linux amd64（Go 1.24.4）平台的完整命令矩阵：所有命令都在干净工作区执行，并通过 `GOMODCACHE=.gomodcache GOPATH=.gopath GOCACHE=.cache/go-build` 约束缓存，运行后立即清理，确保后续平台（Windows/macOS）可直接复现。
- CLI / Go 栈：`go test ./...`、`make linux-custom` 均已通过并重新产出 `bin/HiddifyCli`、`bin/webui`，验证 Stage 5 的 config/extension 改动不会互相踩踏；验证日志附带 CPU/OS / commit hash 以便后续对照。
- Extension：`EXT_HEADLESS=true ./scripts/check-extension.sh` 搭配 `cmd extension --headless` 模式完成 gRPC + Web Server 自检，脚本会校验证书、抓取 HTTPS 200，并在失败时自动 dump `tmp/extension.log`，方便定位。
- CLI run：`./cmd.sh run --config tmp/default-config.json` 现已可直接启动 libbox 服务并写出 `current-config.json`；命令在等待阶段通过 `timeout/CTRL+C` 主动结束，离线环境下出现的 Demo 域名解析/Clash UI 下载告警已记录在验证日志中，属于预期情况。

**阶段 5.3.2 · 冒烟 & 文档**

- Headless extension 冒烟结果（端口、响应码、TLS 指纹）与 CLI 失败日志全部同步写入 `docs/migration-phase5-validation.md`，只需复制表格中的命令即可重现；任何异常会附带“下一步”列，降低沟通成本。
- 与 CLI/extension 交互有关的已知问题（DNS schema 缺口、selector 默认值、`tmp/default-config.json` 过旧等）在该文件里都标注为 Stage 6 工作项，并在本文档交叉引用，方便阅读者理解剩余 gap。
- README / AGENTS.md 目前仍引用旧 sample config，Stage 5.3 的交付明确要求在 Stage 6 的文档任务中一并替换，避免用户继续复制失效模板。

## 阶段 6 · 验证与收尾

- **阶段 6.1 · 配置模板与 CLI 全量运行**
  - ✅ `scripts/generate_default_config` 现会调用 `config.BuildConfigJson` + `config.NormalizeHiddifyOptions`，自动生成 `config/config.json.template`、`config/default.json` 与 `tmp/default-config.json`，同时通过 `injectDemoOutbound` 注入可运行的 Shadowsocks 示例（Base64 密钥）。
  - ✅ `./cmd.sh run --config tmp/default-config.json` 在离线环境下可成功拉起 libbox 服务、保存 `current-config.json` 并进入“等待 CTRL+C”状态；命令由 `timeout`/手动中断退出，日志仅提示 demo 域名解析失败（预期）。
  - ⏳ 若后续引入新的 `hiddify-settings.json` 字段，需要同步 `config.NormalizeHiddifyOptions`、extension UI 默认值以及模板，以避免 CLI / extension 分叉。
- **阶段 6.2 · 全平台命令矩阵**
  - ✅ Linux 侧已完成 `make linux-custom`、`make linux-amd64`、`./cmd.sh run --config tmp/default-config.json`、`./cmd.sh extension`（非 headless，默认监听 12345/12346）与 `./cmd.sh build -c config/config.json.template -o tmp/generated-config.json` 等关键命令，输出均记录在 `docs/migration-phase5-validation.md`。
  - ⏳ Windows/macOS/移动端交叉构建暂未执行。如需补充，请复用相同验证表格并注明所需环境变量/工具链。
- **阶段 6.3 · 手工冒烟场景**
  - ✅ CLI 运行 `./cmd.sh run --config tmp/default-config.json`，可自动生成 `current-config.json` 并保持服务；在离线环境中 demo 域名/Clash UI 下载告警属于预期。
  - ✅ `./cmd.sh extension` + `scripts/check-extension.sh` 验证 gRPC/Web 端口；`./cmd.sh build -c config/config.json.template -o tmp/generated-config.json` 验证 parser 及 selector/url-test 注入流程。操作步骤与日志均写入 Phase 5 验证文档，后续可据此复现/截图。
  - ⏳ 如需体验更多场景（selector UI 切换、Warp/代理检测等），可在现有 CLI/extension 基础上继续手动验证并补充记录。
- **阶段 6.4 · 文档与发布交付**
  - ✅ `docs/migration-phase5-validation.md`/`docs/sing-box-migration.md` 已同步最新验证状态；`go test ./...` 输出亦记录在阶段日志中。
  - ⏳ README、AGENTS.md、release note 仍待统一更新，说明新的模板脚本、pnpm 流程与默认 CLI 行为；完成后即可宣布 Stage 6 收尾。

每个阶段都以“go test 通过”或可复现的检查点结束，确保问题可追踪、可回滚。
