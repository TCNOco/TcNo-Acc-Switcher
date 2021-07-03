var currentVersion = "";

// Returns "Steam" or "Riot" for example, based on the current URL
function getCurrentPage() {
    return (window.location.pathname.split("/")[0] !== "" ?
        window.location.pathname.split("/")[0] :
        window.location.pathname.split("/")[1]);
}

function docReady(fn) {
    // see if DOM is already available
    if (document.readyState === "complete" || document.readyState === "interactive") {
        // call on next available tick
        setTimeout(fn, 1);
    } else {
        document.addEventListener("DOMContentLoaded", fn);
    }
}

// Clear Cache reload: 
var winUrl = window.location.href.split("?");
if (winUrl.length > 1 && winUrl[1].indexOf("cacheReload") !== -1) {
    history.pushState({}, null, window.location.href.replace("cacheReload&", "").replace("cacheReload", ""));
    location.reload(true);
}


function copyToClipboard(str) {
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "CopyToClipboard", str);
}

// FORGETTING ACCOUNTS
async function forget(e) {
    e.preventDefault();
    const reqId = $(selectedElem).attr("id");
    var promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `Get${getCurrentPage()}ForgetAcc`).then((r) => {
        if (!r) showModal(`confirm:AcceptForget${getCurrentPage()}Acc:${reqId}`);
        else Modal_Confirm(`AcceptForget${getCurrentPage()}Acc:${reqId}`, true);
    });
    _ = await promise;

}

// RESTORING STEAM ACCOUNTS
async function restoreSteamAccounts() {
    const reqSteamIds = $("#ForgottenSteamAccounts").children("option:selected").toArray().map((item) => {
        return $(item).attr("value");
    });

    var promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "Steam_RestoreSelected", reqSteamIds).then((r) => {
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

// STOP IGNORING BATTLENET ACCOUNTS
async function restoreBattleNetAccounts() {
    const reqBattleNetId = $("#IgnoredAccounts").children("option:selected").toArray().map((item) => {
        return $(item).attr("value");
    });

    var promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "BattleNet_RestoreSelected", reqBattleNetId).then((r) => {
        if (r === true) {
            reqBattleNetId.forEach((e) => {
                $("#IgnoredAccounts").find(`option[value="${e}"]`).remove();
                window.notification.new({
                    type: "success",
                    title: "",
                    message: "Restored accounts!",
                    renderTo: "toastarea",
                    duration: 5000
                });
            });
        } else {
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

    // Different function groups based on platform
    switch (getCurrentPage()) {
        case "Steam":
            steam();
            break;
        default:
            copyToClipboard(unEscapeString($(selectedElem).attr(request)));
            break;
    }
    return;


    // Steam:
    function steam() {
        var steamId64 = $(selectedElem).attr("id");
        switch (request) {
            case "URL":
                copyToClipboard(`https://steamcommunity.com/profiles/${steamId64}`);
                break;
            case "SteamId32":
            case "SteamId3":
            case "SteamId":
                DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "CopySteamIdType", request, steamId64);
                break;

                // Links
            case "SteamRep":
                copyToClipboard(`https://steamrep.com/search?q=${steamId64}`);
                break;
            case "SteamID.uk":
                copyToClipboard(`https://steamid.uk/profile/${steamId64}`);
                break;
            case "SteamID.io":
                copyToClipboard(`https://steamid.io/lookup/${steamId64}`);
                break;
            case "SteamIDFinder.com":
                copyToClipboard(`https://steamidfinder.com/lookup/${steamId64}/`);
                break;
            default:
                copyToClipboard(unEscapeString($(selectedElem).attr(request)));
        }
    }
}

// Take a string that is HTML escaped, and return a normal string back.
function unEscapeString(s) {
	return s.replace("&lt;", "<").replace("&gt;", ">").replace("&#34;", "\"").replace("&#39;", "'").replace("&#47;", "/");
}


// General function: Get selected account
var selected;
function getSelected() {
	// This may be unnecessary.
	selected = $(".acc:checked");
	if (selected === "" || selected[0] === null || typeof selected[0] === "undefined") {
		return false;
	}
	return true;
}

// Swapping accounts
function swapTo(request, e) {
    if (e !== undefined) e.preventDefault();
    if (!getSelected()) return;

    
    if (request === -1) DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `SwapTo${getCurrentPage()}`, selected.attr("id")); // -1 is for undefined.
    else DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `SwapTo${getCurrentPage()}WithReq`, selected.attr("id"), request);
}

// Swapping accounts
function changeImage(e) {
    if (e !== undefined) e.preventDefault();
    if (!getSelected()) return;
    
    let path = $(".acc:checked").next("label").children("img")[0].getAttribute("src").split('?')[0];
    window.location = window.location + `${(window.location.href.includes("?") ? "&" : "?")}selectImage=${encodeURI(path)}`;
}

// Open Steam\userdata\<steamID> folder
function openUserdata(e) {
	if (e !== undefined) e.preventDefault();
    if (!getSelected()) return;

    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `SteamOpenUserdata`, selected.attr("id"));
}

// Create shortcut for selected icon
function createShortcut(args = '') {
    var selected = $(".acc:checked");
    if (selected === "" || selected[0] === null || typeof selected[0] === "undefined") {
        return;
    }
    var accId = selected.attr("id");

    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "CreateShortcut", getCurrentPage(), accId, selected.attr("Username"), args);
}

function refreshUsername() {
    var selected = $(".acc:checked");
    if (selected === "" || selected[0] === null || typeof selected[0] === "undefined") {
        return;
    }
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "UbisoftRefreshUsername", selected.attr("id"));
}


// NEW LOGIN
function newLogin() {
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `NewLogin_${getCurrentPage()}`);
}

function hidePlatform() {
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "HidePlatform", selectedElem);
}

function createPlatformShortcut() {
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCreatePlatformShortcut", selectedElem);
}

var exportingAccounts = false;

async function exportAllAccounts() {
    if (exportingAccounts) {
	    window.notification.new({
		    type: "error",
            title: "Error",
            message: "Already processing accounts...",
		    renderTo: "toastarea",
            duration: 5000
	    });
        return;
    }
    exportingAccounts = true;
    var promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiExportAccountList", selectedElem).then((r) => {
        let filename = r.split('/')
        saveFile(filename[filename.length - 1], r);
        exportingAccounts = false;
    });
    _ = await promise;
}

function saveFile(fileName, urlFile) {
	let a = document.createElement("a");
	a.style = "display: none";
	document.body.appendChild(a);
	a.href = urlFile;
	a.download = fileName;
	a.click();
	a.remove();
}


// Add currently logged in Origin account
function currentDiscordLogin() {
	showModal("accString:Discord");
}
// Add currently logged in Origin account
function currentEpicLogin() {
	showModal("accString:Epic");
}
// Add currently logged in Origin account
function currentOriginLogin() {
    showModal("accString:Origin");
}
// Add currently logged in Origin account
function currentRiotLogin() {
    showModal("accString:Riot");
}
// Add currently logged in Ubisoft account
function currentUbisoftLogin() {
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "UbisoftAddCurrent");
}



$(".acc").dblclick(() => {
    alert("Handler for .dblclick() called.");
    swapTo();
});
// Link handling
function OpenLinkInBrowser(link) {
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "OpenLinkInBrowser", link);
}


// Info Window
function showModal(modaltype) {
    let input;
    if (modaltype === "info") {
        $("#modalTitle").text("TcNo Account Switcher Information");
        $("#modal_contents").empty();
        currentVersion = "";
        var promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiGetVersion").then((r) => {
            currentVersion = r;
            $("#modal_contents").append(`<div class="infoWindow">
                <div class="imgDiv"><img width="100" margin="5" src="img/TcNo500.png" draggable="false" onclick="OpenLinkInBrowser('https://tcno.co');"></div>
                <div class="rightContent">
                    <h2>TcNo Account Switcher</h2>
                    <p>Created by TechNobo [Wesley Pyburn]</p>
                    <div class="linksList">
                        <a onclick="OpenLinkInBrowser('https://github.com/TcNobo/TcNo-Acc-Switcher');"><svg viewBox="0 0 24 24" draggable="false" alt="GitHub" class="modalIcoGitHub"><use href="img/icons/ico_github.svg#icoGitHub"></use></svg>View on GitHub</a>
                        <a onclick="OpenLinkInBrowser('https://s.tcno.co/AccSwitcherDiscord');"><svg viewBox="0 0 24 24" draggable="false" alt="Discord" class="modalIcoDiscord"><use href="img/icons/ico_discord.svg#icoDiscord"></use></svg>Bug report/Feature request</a>
                        <a onclick="OpenLinkInBrowser('https://tcno.co');"><svg viewBox="0 0 24 24" draggable="false" alt="Website" class="modalIcoNetworking"><use href="img/icons/ico_networking.svg#icoNetworking"></use></svg>Visit tcno.co</a>
                        <a onclick="OpenLinkInBrowser('https://github.com/TcNobo/TcNo-Acc-Switcher/blob/master/DISCLAIMER.md');"><svg viewBox="0 0 2084 2084" draggable="false" alt="GitHub" class="modalIcoDoc"><use href="img/icons/ico_doc.svg#icoDoc"></use></svg>Disclaimer</a>
                    </div>
                </div>
                </div><div class="versionIdentifier"><span>Version: ${currentVersion}</span></div>`);
        });
    } else if (modaltype.startsWith("changeUsername")) {
        // USAGE: "changeUsername"
        Modal_RequestedLocated(false);
        var platformName = modaltype.split(":")[1] ?? "username";
        let extraButtons = "";
        if (getCurrentPage() === "Discord") {
	        extraButtons = `
                <button class="modalOK extra" type="button" id="set_account_name" onclick="discordCopyJS()"><span><svg viewBox="0 0 448 512" draggable="false" alt="C" class="footerIcoInline"><use href="img/fontawesome/copy.svg#img"></use></svg></span></button>
				<button class="modalOK extra" type="button" id="set_account_name" onclick="OpenLinkInBrowser('https://github.com/TcNobo/TcNo-Acc-Switcher/wiki/Platform:-Discord#saving-accounts');"><span><svg viewBox="0 0 384 512" draggable="false" alt="C" class="footerIcoInline"><use href="img/fontawesome/question.svg#img"></use></svg></span></button>`;
        }
        $("#modalTitle").text("Change username");
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div id="modal_contents">
	        <div>
		        <span class="modal-text">Please enter a new ${platformName} for your account${platformName === "username" ? " (Only changes it in the TcNo Account Switcher)" : "."}.</span>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="NewAccountName" style="width: 100%;padding: 8px;" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('change_username').click();">
	        </div>
	        <div class="settingsCol inputAndButton">
				${extraButtons}
		        <button class="modalOK" type="button" id="change_username" onclick="Modal_FinaliseAccNameChange()"><span>Change ${
            platformName}</span></button>
	        </div>
        </div>`);
        input = document.getElementById("NewAccountName");
    } else if (modaltype.startsWith("find:")) {
        // USAGE: "find:<Program_name>:<Program_exe>:<SettingsFile>" -- example: "find:Steam:Steam.exe:SteamSettings"
        platform = modaltype.split(":")[1].replaceAll("_", " ");
        var platformExe = modaltype.split(":")[2];
        var platformSettingsPath = modaltype.split(":")[3];
        Modal_RequestedLocated(false);
        $("#modalTitle").text(`Please locate the ${platform} directory`);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div id="modal_contents">
	        <div style="width: 80vw;">
		        <span class="modal-text">Please enter ${platform}'s directory, as such: C:\\Program Files\\${platform}</span>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="FolderLocation" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('select_location').click();">
		        <button type="button" id="LocateProgramExe" onclick="window.location = window.location + '?selectFile=${platformExe}';"><span>Locate ${platformExe}</span></button>
	        </div>
	        <div class="settingsCol inputAndButton">
		        <div class="folder_indicator notfound"><div id="folder_indicator_text"></div></div>
		        <div class="folder_indicator_bg notfound"><span>${platformExe}</span></div>
		        <button class="modalOK" type="button" id="select_location" onclick="Modal_Finalise('${platform}', '${platformSettingsPath
            }')"><span>Select ${platform} Folder</span></button>
	        </div>
        </div>`);
        input = document.getElementById("FolderLocation");
    } else if (modaltype.startsWith("confirm:")) {
        // USAGE: "confirm:<prompt>
        // GOAL: To return true/false
        let action = modaltype.slice(8);

        let message = "";
        let header = "<h3>Confirm action:</h3>";
        if (action.startsWith("AcceptForgetSteamAcc")) {
            message = forgetAccountSteamPrompt;
        } else if (action.startsWith("AcceptForgetDiscordAcc") || action.startsWith("AcceptForgetEpicAcc")
	        || action.startsWith("AcceptForgetOriginAcc") || action.startsWith("AcceptForgetUbisoftAcc") ||
            action.startsWith("AcceptForgetBattleNetAcc") || action.startsWith("AcceptForgetRiotAcc")) {
            message = getAccountPrompt();
        } else {
            message = `<p>${modaltype.split(":")[2].replaceAll("_", " ")}</p>`;
            // The only exception to confirm:<prompt> was AcceptForgetSteamAcc, as that was confirm:AcceptForgetSteamAcc:steamId
            // Could be more in the future.
            action = action.split(":")[0];
        }

        $("#modalTitle").text("TcNo Account Switcher Confirm Action");
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div class="infoWindow">
        <div class="fullWidthContent">${header + message}
            <div class="YesNo">
		        <button type="button" id="modal_true" onclick="Modal_Confirm('${action}', true)"><span>Yes</span></button>
		        <button type="button" id="modal_false" onclick="Modal_Confirm('${action}', false)"><span>No</span></button>
            </div>
        </div>
        </div>`);
    } else if (modaltype.startsWith("notice:")) {
        // USAGE: "notice:<prompt>
        // GOAL: Runs function when OK clicked.
        let action = modaltype.slice(7);
        let args = "";
        if (modaltype.split(":").length > 2) {
            action = modaltype.slice(7).split(":")[0];
            args = modaltype.slice(7).split(":")[1];
        }

        let message = "";
        let header = "<h3>Confirm action:</h3>";
        if (action.startsWith("RestartAsAdmin")) {
            message = restartAsAdminPrompt;
            action = (args !== "" ? `location = 'RESTART_AS_ADMIN?arg=${args}'` : "location = 'RESTART_AS_ADMIN'");
        } else {
            message = `<p>${modaltype.split(":")[2].replaceAll("_", " ")}</p>`;
            action = action.split(":")[0];
        }

        $("#modalTitle").text("TcNo Account Switcher Confirm Action");
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div class="infoWindow">
        <div class="fullWidthContent">${header + message}
            <div class="YesNo">
		        <button type="button" id="modal_true" onclick="${action}"><span>OK</span></button>
            </div>
        </div>
        </div>`);
        input = document.getElementById("modal_true");
    } else if (modaltype.startsWith("accString:")) {
        // USAGE: "accString:<platform>" -- example: "accString:Origin"
        platform = modaltype.split(":")[1].replaceAll("_", " ");
        let extraButtons = "";
        if (platform === "Discord") {
            extraButtons = `
                <button class="modalOK extra" type="button" id="set_account_name" onclick="discordCopyJS()"><span><svg viewBox="0 0 448 512" draggable="false" alt="C" class="footerIcoInline"><use href="img/fontawesome/copy.svg#img"></use></svg></span></button>
				<button class="modalOK extra" type="button" id="set_account_name" onclick="OpenLinkInBrowser('https://github.com/TcNobo/TcNo-Acc-Switcher/wiki/Platform:-Discord#saving-accounts');"><span><svg viewBox="0 0 384 512" draggable="false" alt="C" class="footerIcoInline"><use href="img/fontawesome/question.svg#img"></use></svg></span></button>`;
        }
        Modal_RequestedLocated(false);
        $("#modalTitle").text(`Add new ${platform} account`);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div id="modal_contents">
	        <div>
		        <span class="modal-text">Please enter a name for the ${platform} account you're logged into.</span>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="CurrentAccountName" style="width: 100%;padding: 8px;" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('set_account_name').click();" onkeypress='return /[^<>:\\.\\"\\/\\\\|?*]/i.test(event.key)'>
	        </div>
	        <div class="settingsCol inputAndButton">
				${extraButtons}
		        <button class="modalOK" type="button" id="set_account_name" onclick="Modal_FinaliseAccString('${platform}')"><span>Add current ${platform} account</span></button>
	        </div>
        </div>`);
        input = document.getElementById("CurrentAccountName");
    } else {
        $("#modalTitle").text("Notice");
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div class="infoWindow"><div class="fullWidthContent">${modaltype}</div></div>`);
    }
    $(".modalBG").fadeIn(() => {
        if (input === undefined) return;
        try {
            input.focus();
            input.select();
        }
        catch (err) {

        }
    });
}

function Modal_SetFilepath(path) {
    $("#FolderLocation").val(path);
}
// For finding files with modal:
function Modal_RequestedLocated(found) {
    $(".folder_indicator").removeClass("notfound found");
    $(".folder_indicator_bg").removeClass("notfound found");
    if (found === true) {
        $(".folder_indicator").addClass("found");
        $(".folder_indicator_bg").addClass("found");
    } else {
        $(".folder_indicator").addClass("notfound");
        $(".folder_indicator_bg").addClass("notfound");
    }
}

function Modal_Finalise(platform, platformSettingsPath) {
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiUpdatePath", platformSettingsPath, $("#FolderLocation").val());
    $(".modalBG").fadeOut();
    location.reload();
}
async function Modal_Confirm(action, value) {
    var promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiConfirmAction", action, value).then((r) => {
        if (r === "refresh") location.reload();
    });
    var result = await promise;
    $(".modalBG").fadeOut();
}

function Modal_FinaliseAccString(platform) {
    // Supported: Discord, Epic, Origin, Riot
    let raw = $("#CurrentAccountName").val();
    let name = (raw.indexOf("TCNO:") === -1 ? raw.replace(/[<>: \.\"\/\\|?*]/g, "-") : raw); // Clean string if not a command string.
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", platform + "AddCurrent", name);
    $(".modalBG").fadeOut();
    $("#acc_list").click();
}

function Modal_FinaliseAccNameChange() {
    let raw = $("#NewAccountName").val();
	let name = (raw.indexOf("TCNO:") === -1 ? raw.replace(/[<>: \.\"\/\\|?*]/g, "-") : raw); // Clean string if not a command string.
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "ChangeUsername", $(".acc:checked").attr("id"), name, getCurrentPage());
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

function forgetBattleTag() {
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "DeleteUsername", $(".acc:checked").attr("id"));
}

function refetchRank() {
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "RefetchRank", $(".acc:checked").attr("id"));
}

const forgetAccountSteamPrompt = `<h3 style='color:red'>You are about to forget an account!</h3>
<h4>What does this mean?</h4>
<p>- Steam will no longer have the account listed in Big Picture Mode and will not Remember Password.<br/>
- TcNo Account Switcher will also no longer show the account, until it's signed into again through Steam.</p>
<p>Your account will remain untouched. It is just forgotten on this computer.</p>
<h4>What if something goes wrong?</h4>
<p>Don't panic, you can bring back forgotten accounts via backups in the Settings screen.<br/>
You can also remove previous backups from there when you are sure everything is working as expected.</p>
<h4>Do you understand?</h4>`;


// Find a better way to display these. A placeholder that gets replaced for the platform name?
function getAccountPrompt() {
    return `<h3 style='color:red'>You are about to forget an account!</h3>
<h4>What does this mean?</h4>
<p>TcNo Account Switcher will also no longer show the account,<br/>
until it's signed into again through ${getCurrentPage()}, and added to the list.</p>
<p>Your account will remain untouched. It is just forgotten on this computer.</p>
<h4>Do you understand?</h4>`;
}

const restartAsAdminPrompt = `<h3><bold>This program will restart as Admin</bold></h3>
<p>Hit "Yes" in UAC when prompted for admin.</p>`;


function discordCopyJS() {
    // Clicks the User Settings button
    // Then immediately copies the 'src' of the profile image, and the username, as well as the #.
    // Then copy to clipboard.
    let code = `
// Find options button, and click
let btns = document.getElementsByClassName("button-14-BFJ");
btns[btns.length-1].click();

// Check that streamer mode is not enabled, otherwise: Give error.
if ($("[class^='streamerModeEnabledBtn']") !== null){
console.log.apply(console, ["%cTcNo Account Switcher%c: ERROR!\\nMake sure that Streamer Mode is disabled/not active when running this command!\\n%chttps://github.com/TcNobo/TcNo-Acc-Switcher/wiki/Platform:-Discord#saving-accounts ", 'background: #290000; color: #F00','background: #290000; color: white','background: #222; color: lightblue']);
}else{
  // Get Avatar and username from page
  let avatar = $("[class^='userInfo']").getElementsByTagName("img")[0].src;
  let name = $("[class^='userInfo']").getElementsByTagName("span")[0].innerText + $("[class^='userInfo']").getElementsByTagName("span")[1].innerText;

  // Copy avatar and username
  copy(\`TCNO: \${avatar}|\${name}\`);

  // Close options
  $("[class^='closeButton']").click();

  // Let the user know in console.
  console.log.apply(console, ["%cTcNo Account Switcher%c: Successfully copied information!\\nPaste it into the input box in the account switcher to update/set image and username.\\n%chttps://github.com/TcNobo/TcNo-Acc-Switcher/wiki/Platform:-Discord#saving-accounts ", 'background: #222; color: #bada55','background: #222; color: white','background: #222; color: lightblue']);
}`;
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "CopyToClipboard", code);
    window.notification.new({
	    type: "success",
	    title: "Copied",
	    message: "Paste into Discord console, and then paste result in input!",
	    renderTo: "toastarea",
	    duration: 5000
    });
}