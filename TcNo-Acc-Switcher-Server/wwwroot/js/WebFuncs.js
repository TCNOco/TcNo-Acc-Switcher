
var currentpage = "Steam";

function CopyToClipboard(str) {
    DotNet.invokeMethodAsync('TcNo-Acc-Switcher', "CopyToClipboard", str);
}

function copy(request) {
    var RequestResult = $(SelectedElem).attr(request);

    if (RequestResult == null) {
        // Different function groups based on platform
        switch (currentpage) {
            case "Steam":
                Steam();
        default:
        }
    } else {
        console.log(`Copying: ${request}, result: ${RequestResult}`);
        CopyToClipboard(RequestResult).then(r => console.log(r));
    }


    // Steam:
    function Steam() {
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




//DotNet.invokeMethodAsync('TcNo-Acc-Switcher', "CopyCommunityUsername", $(SelectedElem).attr(request)).then(r => console.log(r));