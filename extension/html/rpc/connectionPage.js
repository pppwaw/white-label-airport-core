const {  hiddifyClient } = require('./client.js');
const hiddify = require("./hiddify_grpc_web_pb.js");

async function hydrateDefaults() {
    await Promise.allSettled([loadHiddifySettings(), loadCapabilities()]);
}

async function loadHiddifySettings() {
    const request = new hiddify.Empty();
    try {
        const response = await hiddifyClient.getHiddifySettings(request, {});
        const currentValue = ($("#hiddify-settings").val() || "").trim();
        if (!currentValue && response.getHiddifySettingsJson()) {
            $("#hiddify-settings").val(response.getHiddifySettingsJson());
        }
    } catch (err) {
        console.error("Failed to fetch Hiddify settings", err);
    }
}

async function loadCapabilities() {
    const request = new hiddify.Empty();
    const capabilityList = $("#capability-list");
    try {
        const response = await hiddifyClient.getConfigCapabilities(request, {});
        const rows = [
            `TLS Fragmentation: ${response.getSupportsTlsFragment() ? "supported" : "not available"}`,
            `QUIC: ${response.getSupportsQuic() ? "supported" : "not available"}`,
            `ECH: ${response.getSupportsEch() ? "supported" : "not available"}`,
            `Schema Version: ${response.getSchemaVersion() || "unknown"}`
        ];
        capabilityList.empty();
        rows.forEach((line) => capabilityList.append($("<li>").text(line)));
    } catch (err) {
        capabilityList.empty().append($("<li>").text("Unable to load capabilities"));
        console.error("Failed to load capabilities", err);
    }
}

function openConnectionPage() {
    
        $("#extension-list-container").show();
        $("#extension-page-container").hide();
        $("#connection-page").show();
        connect();
        hydrateDefaults();
        $("#connect-button").click(async () => {
            const hsetting_request = new hiddify.ChangeHiddifySettingsRequest();
            hsetting_request.setHiddifySettingsJson($("#hiddify-settings").val());
            try{
                const hres=await hiddifyClient.changeHiddifySettings(hsetting_request, {});
            }catch(err){
                $("#hiddify-settings").val("")
                console.log(err)
            }
            
            const parse_request = new hiddify.ParseRequest();
            parse_request.setContent($("#config-content").val());
            try{
                const pres=await hiddifyClient.parse(parse_request, {});
                if (pres.getResponseCode() !== hiddify.ResponseCode.OK){
                    alert(pres.getMessage());
                    return
                }
                $("#config-content").val(pres.getContent());
            }catch(err){
                console.log(err)
                alert(JSON.stringify(err))
                                return
            }

            const request = new hiddify.StartRequest();
    
            request.setConfigContent($("#config-content").val());
            request.setEnableRawConfig(false);
            try{
                const res=await hiddifyClient.start(request, {});
                console.log(res.getCoreState(),res.getMessage())
                    handleCoreStatus(res.getCoreState());
            }catch(err){
                console.log(err)
                alert(JSON.stringify(err))
                return
            }

            
        })

        $("#disconnect-button").click(async () => {
            const request = new hiddify.Empty();
            try{
                const res=await hiddifyClient.stop(request, {});
                console.log(res.getCoreState(),res.getMessage())
                handleCoreStatus(res.getCoreState());
            }catch(err){
                console.log(err)
                alert(JSON.stringify(err))
                return
            }
        })
}


function connect(){
    const request = new hiddify.Empty();
    const stream = hiddifyClient.coreInfoListener(request, {});
    stream.on('data', (response) => {
        console.log('Receving ',response);
        handleCoreStatus(response);
    });
    
    stream.on('error', (err) => {
        console.error('Error opening extension page:', err);
        // openExtensionPage(extensionId);
    });
    
    stream.on('end', () => {
        console.log('Stream ended');
        setTimeout(connect, 1000);
        
    });
}


function handleCoreStatus(status){
    if (status == hiddify.CoreState.STOPPED){
        $("#connection-before-connect").show();
        $("#connection-connecting").hide();
    }else{
        $("#connection-before-connect").hide();
        $("#connection-connecting").show();
        if (status == hiddify.CoreState.STARTING){
            $("#connection-status").text("Starting");
            $("#connection-status").css("color", "yellow");
        }else if (status == hiddify.CoreState.STOPPING){
            $("#connection-status").text("Stopping");
            $("#connection-status").css("color", "red");
        }else if (status == hiddify.CoreState.STARTED){
            $("#connection-status").text("Connected");
            $("#connection-status").css("color", "green");
        }
    }
}


module.exports = { openConnectionPage };
