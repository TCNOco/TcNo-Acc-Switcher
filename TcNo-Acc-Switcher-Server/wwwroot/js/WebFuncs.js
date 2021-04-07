var currentVersion = 0001;


// This will be changed later on to NOT be in this file, as each different account switcher will have their own file.
var currentpage = "Steam";

// Clear Cache reload: 
var winUrl = window.location.href.split("?");
if (winUrl.length > 1 && winUrl[1].contains("cacheReload")) {
    history.pushState({}, null, window.location.href.replace("cacheReload&", "").replace("cacheReload", ""));
    location.reload(true);
}


function CopyToClipboard(str) {
    DotNet.invokeMethodAsync('TcNo-Acc-Switcher-Server', "CopyToClipboard", str);
}

// FORGETTING STEAM ACCOUNTS
function forget(e) {
    e.preventDefault();
    switch (currentpage) {
        case "Steam":
            promptForgetSteam();
        default:
    }
}
async function promptForgetSteam() {
    const reqSteamId = $(SelectedElem).attr("steamid64");
    
    var promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GetSteamForgetAcc").then(r => {
        if (!r) ShowModal("confirm:AcceptForgetSteamAcc:" + reqSteamId);
        else forgetSteam(reqSteamId);
    });
    var result = await promise;
}
async function forgetSteam(reqSteamId) {
    var promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "ForgetAccountJs", reqSteamId).then(
        _ => {
            location.reload();
        });
    var result = await promise;
}

// RESTORING STEAM ACCOUNTS
async function restoreSteamAccounts() {
    //const reqSteamId = $("#ForgottenSteamAccounts").children("option:selected")
    //    .each((_, e) => { console.log($(e).attr("value")) });
    const reqSteamIds = $("#ForgottenSteamAccounts").children("option:selected").toArray().map((item) => {
        return $(item).attr("value");
    });

    var promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "Steam_RestoreSelected", reqSteamIds).then(r => {
        if (r === true) {
            reqSteamIds.forEach((e) => {
                $("#ForgottenSteamAccounts").find(`option[value="${e}"]`).remove();
                window.notification.new({
                    type: "success",
                    title: "",
                    message: "Restored accounts!",
                    renderTo: "toastarea",
                    duration: 5000
                });
            });
        } else {
            console.log(r);
            window.notification.new({
                type: "error",
                title: "",
                message: "Failed to restore accounts (See console)",
                renderTo: "toastarea",
                duration: 5000
            });
        }
    });
    var result = await promise;
}

function copy(request, e) {
    e.preventDefault();
    const requestResult = $(SelectedElem).attr(request);
    
    // Different function groups based on platform
    switch (currentpage) {
        case "Steam":
            steam();
        default:
            CopyToClipboard(requestResult);
    }
    return;


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
                DotNet.invokeMethodAsync('TcNo-Acc-Switcher-Server', "CopySteamIdType", request, steamId64);
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
                CopyToClipboard(requestResult);
        }
    }
}

// Swapping accounts
function SwapTo(request, e) {
    if (e !== undefined) e.preventDefault();

    // Different function groups based on platform
    switch (currentpage) {
        case "Steam":
            steam();
        default:
    }

    //Steam: 
    function steam() {
        // This may be unnecessary.
        var selected = $(".acc:checked");
        if (selected === "" || selected[0] === null || typeof selected[0] === "undefined") { return; }

        var steamId64 = $(SelectedElem).attr("SteamID64");
        DotNet.invokeMethodAsync('TcNo-Acc-Switcher-Server', "SwapTo", selected.attr("SteamID64"), selected.attr("Username"), request);
    }
}

// Create shortcut for selected icon
function CreateShortcut() {
    var selected = $(".acc:checked");
    if (selected === "" || selected[0] === null || typeof selected[0] === "undefined") { return; }
    DotNet.invokeMethodAsync('TcNo-Acc-Switcher-Server', "CreateShortcut", selected.attr("SteamID64"), selected.attr("Username"));
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
        </div><div class="versionIdentifier"><span>Version: 00001</span></div>`);
    }
    else if (modaltype.startsWith("find:")) {
        // USAGE: "find:<Program_name>:<Program_exe>:<SettingsFile>" -- example: "find:Steam:Steam.exe:SteamSettings"
        console.log(modaltype);
        console.log(modaltype.split(":"));
        var platform = modaltype.split(":")[1].replaceAll("_", " ");
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
    } else if (modaltype.startsWith("confirm:")) {
        // USAGE: "confirm:<prompt>
        // GOAL: To return true/false
        console.log(modaltype);

        let action = modaltype.slice(8);

        let message = "";
        let header = "";
        if (action.startsWith("AcceptForgetSteamAcc")) {
            message = forgetAccountSteamPrompt;
        } else {
            header = "<h3>Confirm action:</h3>";
            message = "<p>" + modaltype.split(":")[2].replaceAll("_", " ") + "</p>";
            // The only exception to confirm:<prompt> was AcceptForgetSteamAcc, as that was confirm:AcceptForgetSteamAcc:steamId
            // Could be more in the future.
            action = action.split(":")[0];
        }

        $('#modalTitle').text("TcNo Account Switcher Confirm Action");
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div class="infoWindow">
        <div class="fullWidthContent">
            ` + header + `
            ` + message + `
            <div class="YesNo">
		        <button class="btn" type="button" id="modal_true" onclick="Modal_Confirm('` + action + `', true)"><span>Yes</span></button>
		        <button class="btn" type="button" id="modal_false" onclick="Modal_Confirm('` + action + `', false)"><span>No</span></button>
            </div>
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
async function Modal_Confirm(action, value) {
    var promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiConfirmAction", action, value).then(r => {
        if (r === "refresh") location.reload();
    });
    var result = await promise;
    $('.modalBG').fadeOut();
}


var appendDelay = 100; // Milliseconds
var recentlyAppend = false;
var pendingQueue = {};
function queuedJQueryAppend(jQuerySelector, strToInsert) {
    if (recentlyAppend) {
        if (jQuerySelector in pendingQueue) pendingQueue[jQuerySelector] += strToInsert;
        else pendingQueue[jQuerySelector] = strToInsert;
    } else {
        recentlyAppend = true;
        setTimeout(flushJQueryAppendQueue, appendDelay);
        $(jQuerySelector).append([strToInsert]);
        // have this as detect and run at some point. For now the only use for this function is the Steam Cleaning list thingy
        $(".clearingRight")[0].scrollTop = $(".clearingRight")[0].scrollHeight;
    }
}
function flushJQueryAppendQueue() {
    for (const [key, value] of Object.entries(pendingQueue)) {
        $(key).append(value);
    }
    pendingQueue = {};
    recentlyAppend = false;
    // have this as detect and run at some point. For now the only use for this function is the Steam Cleaning list thingy
    $(".clearingRight")[0].scrollTop = $(".clearingRight")[0].scrollHeight;
}

//DotNet.invokeMethodAsync('TcNo-Acc-Switcher-Server', "CopyCommunityUsername", $(SelectedElem).attr(request)).then(r => console.log(r));




const forgetAccountSteamPrompt = `<h3 style='color:red'>You are about to forget an account!</h3>
<h4>What does this mean?</h4>
<p>- Steam will no longer have the account listed in Big Picture Mode and will not Remember Password.<br/>
- TcNo Account Switcher will also no longer show the account, until it's signed into again through Steam.</p>
<p>Your account will remain untouched. It is just forgotten on this computer.</p>
<h4>What if something goes wrong?</h4>
<p>Don't panic, you can bring back forgotten accounts via backups in the Settings screen.<br/>
You can also remove previous backups from there when you are sure everything is working as expected.</p>
<p>Right-click &gt;&gt; Forget and using the Delete key will both work.</p>
<h4>Do you understand?</h4>`;