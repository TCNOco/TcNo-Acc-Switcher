
var currentpage = "Steam";

function CopyToClipboard(str) {
    DotNet.invokeMethodAsync('TcNo-Acc-Switcher', "CopyToClipboard", str);
}

function copy(request) {
    var requestResult = $(SelectedElem).attr(request);

    if (requestResult == null) {
        // Different function groups based on platform
        switch (currentpage) {
            case "Steam":
                steam();
        default:
        }
    } else {
        console.log(`Copying: ${request}, result: ${requestResult}`);
        CopyToClipboard(requestResult).then(r => console.log(r));
    }


    // Steam:
    function steam() {
        var steamId64 = $(SelectedElem).attr("SteamID64");
        switch (request){
            case "URL":
                //DotNet.invokeMethodAsync('TcNo-Acc-Switcher', "CopySpecial", request);
                CopyToClipboard("https://steamcommunity.com/profiles/" + steamId64);
                break;
            case "SteamId32":
            case "SteamId3":
            case "SteamId":
                DotNet.invokeMethodAsync('TcNo-Acc-Switcher', "CopySteamIDType", request, steamId64);
                break;

            // Links
            case "SteamRep":
                CopyToClipboard(`https://steamrep.com/search?q=${steamId64}`);
                break;
            case "SteamID.uk":
                CopyToClipboard(`https://steamid.uk/profile/${steamId64}`);
                break;
            case "SteamID.io":
                CopyToClipboard(`https://steamid.io/lookup/${steamId64}`);
                break;
            case "SteamIDFinder.com":
                CopyToClipboard(`https://steamidfinder.com/lookup/${steamId64}/`);
                break;

            default:

        }
    }
}
// Swapping accounts
function SwapTo() {
    var selected = $(".acc:checked");
    if (selected === "" || selected[0] === null || typeof selected[0] === "undefined") { return; }
    var steamId64 = selected.attr("SteamID64");
    var accName = selected.attr("Username");
    DotNet.invokeMethodAsync('TcNo-Acc-Switcher', "SwapTo", steamId64, accName);
}
$(".acc").dblclick(function () {
    alert("Handler for .dblclick() called.");
    SwapTo();
});




//DotNet.invokeMethodAsync('TcNo-Acc-Switcher', "CopyCommunityUsername", $(SelectedElem).attr(request)).then(r => console.log(r));