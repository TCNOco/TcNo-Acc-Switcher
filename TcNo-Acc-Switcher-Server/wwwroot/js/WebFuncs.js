
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
// Link handling
function OpenLinkInBrowser(link) {
    DotNet.invokeMethodAsync('TcNo-Acc-Switcher', "OpenLinkInBrowser", link);
}


// Info Window
function ShowModal(modaltype) {
    if (modaltype == "info") {
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div class="infoWindow">
        <div class= "imgDiv" ><img width="100" margin="5" src="img/TcNo500.png"></div>
        <div class="rightContent">
            <h2>TcNo Account Switcher</h2>
            <p>Created by TechNobo [Wesley Pyburn]</p>
            <div class="linksList">
                <a onclick="OpenLinkInBrowser('https://github.com/TcNobo/TcNo-Acc-Switcher');"><img src="img/icons/ico_discord.svg" draggable="false">View on GitHub</a>
                <a onclick="OpenLinkInBrowser('https://s.tcno.co/AccSwitcherDiscord');"><img src="img/icons/ico_github.svg" draggable="false">Bug report/Feature request</a>
                <a onclick="OpenLinkInBrowser('https://tcno.co');"><img src="img/icons/ico_networking.svg" draggable="false">Visit tcno.co</a>
            </div>
        </div>
    </div>`);
    }
    $('.modalBG').fadeIn();
}

//DotNet.invokeMethodAsync('TcNo-Acc-Switcher', "CopyCommunityUsername", $(SelectedElem).attr(request)).then(r => console.log(r));