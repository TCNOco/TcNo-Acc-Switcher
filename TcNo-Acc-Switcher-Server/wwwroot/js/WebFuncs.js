
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
    DotNet.invokeMethodAsync('TcNo-Acc-Switcher-Server', "SwapTo", steamId64, accName);
}

// New Steam accounts
function NewSteamLogin() {
    DotNet.invokeMethodAsync('TcNo-Acc-Switcher-Server', "SwapTo", "", "");
}

$(".acc").dblclick(function () {
    alert("Handler for .dblclick() called.");
    SwapTo();
});
// Link handling
function OpenLinkInBrowser(link) {
    DotNet.invokeMethodAsync('TcNo-Acc-Switcher-Server', "OpenLinkInBrowser", link);
}


// Info Window
function ShowModal(modaltype) {
    if (modaltype == "info") {
        $('#modalTitle').text("TcNo Account Switcher Information");
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div class="infoWindow">
        <div class= "imgDiv" ><img width="100" margin="5" src="img/TcNo500.png" draggable="false"></div>
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
    else if (modaltype.startsWith("find:")) {
        // USAGE: "find:<Program_name>:<Program_exe>:<SettingsFile>" -- example: "find:Steam:Steam.exe:SteamSettings"
        console.log(modaltype);
        console.log(modaltype.split(":"));
        var platform = modaltype.split(":")[1].replace("_", " ");
        var platform_exe = modaltype.split(":")[2];
        var platformSettingsPath = modaltype.split(":")[3];
        Modal_RequestedLocated(false);

        $('#modalTitle').text("Please locate the " + platform + " directory");
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div id="modal_contents">
	        <div style="width: 80vw;">
		        <span class="modal-text">Please enter ` + platform + `'s directory, as such: C:\\Program Files\\` + platform + `</span>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="FolderLocation">
		        <button class="btn" type="button" id="LocateProgramExe" onclick="window.location = window.location + '?selectFile=` + platform_exe + `';"><span>Locate ` + platform_exe + `"</span></button>
	        </div>
	        <div class="settingsCol inputAndButton">
		        <div class="folder_indicator notfound"><div id="folder_indicator_text"></div></div>
		        <div class="folder_indicator_bg notfound"><span>` + platform_exe + `</span></div>
		        <button class="btn" type="button" id="select_program" onclick="Modal_Finalize('` + platform + `', '` + platformSettingsPath + `')"><span>Select ` + platform + ` Folder</span></button>
	        </div>
        </div>`);
    }
    $('.modalBG').fadeIn();
}

function Modal_SetFilepath(path) { $("#FolderLocation").val(path); }
// For finding files with modal:
function Modal_RequestedLocated(found) {
    $(".folder_indicator").removeClass("notfound found");
    $(".folder_indicator_bg").removeClass("notfound found");
    if (found == true) {
        $(".folder_indicator").addClass("found");
        $(".folder_indicator_bg").addClass("found");
    } else {
        $(".folder_indicator").addClass("notfound");
        $(".folder_indicator_bg").addClass("notfound");
    }
}
function Modal_Finalize(platform, platformSettingsPath) {
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiUpdatePath", platformSettingsPath, $("#FolderLocation").val());
    $('.modalBG').fadeOut();
    $('#Switcher' + platform).click();
}
//DotNet.invokeMethodAsync('TcNo-Acc-Switcher-Server', "CopyCommunityUsername", $(SelectedElem).attr(request)).then(r => console.log(r));