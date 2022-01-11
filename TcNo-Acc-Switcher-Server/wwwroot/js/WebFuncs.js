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

async function GetLang(k) {
    return DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiLocale", k).then((r) => {
        return r;
    });
}
async function GetCrowdin() {
    return DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCrowdinList").then((r) => {
        return r;
    });
}
async function GetLangSub(key, obj) {
    return DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiLocaleObj", key, obj).then((r) => {
        return r;
    });
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
	const toastFailedRestore = await GetLang("Toast_FailedRestore"),
		toastRestored = await GetLang("Toast_Restored");
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
                    message: toastRestored,
                    renderTo: "toastarea",
                    duration: 5000
                });
            });
        } else {
	        window.notification.new({
                type: "error",
                title: "",
                message: toastFailedRestore,
                renderTo: "toastarea",
                duration: 5000
            });
        }
    });
    var result = await promise;
}

// STOP IGNORING BATTLENET ACCOUNTS
async function restoreBattleNetAccounts() {
	const toastFailedRestore = await GetLang("Toast_FailedRestore"),
		toastRestored = await GetLang("Toast_Restored");
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
                    message: toastRestored,
                    renderTo: "toastarea",
                    duration: 5000
                });
            });
        } else {
	        window.notification.new({
                type: "error",
                title: "",
                message: toastFailedRestore,
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
    const selected = $(".acc:checked");
    if (selected === "" || selected[0] === null || typeof selected[0] === "undefined") {
        return;
    }
    const accId = selected.attr("id");

    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server",
        "CreateShortcut",
        getCurrentPage(),
        accId,
        selected.attr("Username"),
        args);
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
	    const toastAlreadyProcessing = await GetLang("Toast_AlreadyProcessing"),
		    error = await GetLang("Error");

	    window.notification.new({
		    type: "error",
            title: error,
            message: toastAlreadyProcessing,
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
async function currentDiscordLogin() {
    // Check to see if it is necessary first
    var skipQuestion = false;
    var promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SkipGetUsername").then((r) => {
	    skipQuestion = r;
    });
    _ = await promise;

    if (!skipQuestion)
		showModal("accString");
}
// Add currently logged in Ubisoft account
async function currentUbisoftLogin() {
    var promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "UbisoftHasUserSaved").then((r) => {
        if (r === "")
            // If userId not saved:
            showModal('accString');
        else {
            DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "UbisoftAddCurrent", r);
        }
    });
    var result = await promise;
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
async function showModal(modaltype) {
    let input, platform;
    if (modaltype === "info") {
	    const modalInfoCreator = await GetLang("Modal_Info_Creator"),
		    modalInfoVersion = await GetLang("Modal_Info_Version"),
		    modalInfoDisclaimer = await GetLang("Modal_Info_Disclaimer"),
		    modalInfoVisitSite = await GetLang("Modal_Info_VisitSite"),
		    modalInfoBugReport = await GetLang("Modal_Info_BugReport"),
            modalInfoViewGitHub = await GetLang("Modal_Info_ViewGitHub"),
            modalTitleInfo = await GetLang("Modal_Title_Info");

        $("#modalTitle").text(modalTitleInfo);
        $("#modal_contents").empty();
        currentVersion = "";

        DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiGetVersion").then((r) => {
            currentVersion = r;
            $("#modal_contents").append(`<div class="infoWindow">
                <div class="imgDiv"><img width="100" margin="5" src="img/TcNo500.png" draggable="false" onclick="OpenLinkInBrowser('https://tcno.co');"></div>
                <div class="rightContent">
                    <h2>TcNo Account Switcher</h2>
                    <p>${modalInfoCreator}</p>
                    <div class="linksList">
                        <a onclick="OpenLinkInBrowser('https://github.com/TcNobo/TcNo-Acc-Switcher');"><svg viewBox="0 0 24 24" draggable="false" alt="GitHub" class="modalIcoGitHub"><use href="img/icons/ico_github.svg#icoGitHub"></use></svg>${modalInfoViewGitHub}</a>
                        <a onclick="OpenLinkInBrowser('https://s.tcno.co/AccSwitcherDiscord');"><svg viewBox="0 0 24 24" draggable="false" alt="Discord" class="modalIcoDiscord"><use href="img/icons/ico_discord.svg#icoDiscord"></use></svg>${modalInfoBugReport}</a>
                        <a onclick="OpenLinkInBrowser('https://tcno.co');"><svg viewBox="0 0 24 24" draggable="false" alt="Website" class="modalIcoNetworking"><use href="img/icons/ico_networking.svg#icoNetworking"></use></svg>${modalInfoVisitSite}</a>
                        <a onclick="OpenLinkInBrowser('https://github.com/TcNobo/TcNo-Acc-Switcher/blob/master/DISCLAIMER.md');"><svg viewBox="0 0 2084 2084" draggable="false" alt="GitHub" class="modalIcoDoc"><use href="img/icons/ico_doc.svg#icoDoc"></use></svg>${modalInfoDisclaimer}</a>
                    </div>
                </div>
                </div><div class="versionIdentifier"><span>${modalInfoVersion}: ${currentVersion}</span></div>`);
        });
    } else if (modaltype === "crowdin") {
        const modalCrowdinHeader = await GetLang("Modal_Crowdin_Header"),
            modalCrowdinInfo = await GetLang("Modal_Crowdin_Info"),
            listUsers = await GetCrowdin();
        $("#modalTitle").text(modalCrowdinHeader);
        $("#modal_contents").empty();

        $("#modal_contents").append(`<div class="infoWindow">
            <div class="fullWidthContent crowdin">
                <h2>${modalCrowdinHeader}<svg viewBox="0 0 512 512" draggable="false" alt="<3" class="heart"><use href="img/fontawesome/heart.svg#img"></use></svg></h2>
                    <p>${modalCrowdinInfo}</p>
                    <ul>${listUsers}</ul>
            </div></div>`);
    } else if (modaltype.startsWith("changeUsername")) {
        // USAGE: "changeUsername"
        Modal_RequestedLocated(false);
        var platformName = modaltype.split(":")[1] ?? "username";
        let extraButtons = "";
        if (getCurrentPage() === "Discord") {
            extraButtons = `
                <button class="modalOK extra" type="button" id="set_account_name" onclick="discordCopyJS()"><span><svg viewBox="0 0 448 512" draggable="false" alt="C" class="footerIcoInline"><use href="img/fontawesome/copy.svg#img"></use></svg></span></button>
				<button class="modalOK extra" type="button" id="set_account_name" onclick="OpenLinkInBrowser('https://github.com/TcNobo/TcNo-Acc-Switcher/wiki/Platform:-Discord#adding-accounts-to-the-discord-switcher-list');"><span><svg viewBox="0 0 384 512" draggable="false" alt="C" class="footerIcoInline"><use href="img/fontawesome/question.svg#img"></use></svg></span></button>`;
        }

        let platformText = (platformName === "username" ? " (Only changes it in the TcNo Account Switcher)" : "");
        const modalChangeUsername =
                  await GetLangSub("Modal_ChangeUsername", { platformText: platformName, optional: platformText }),
            modalChangeUsernameType = await GetLangSub("Modal_ChangeUsernameType", { UsernameOrOther: platformName }),
            modalTitleChangeUsername = await GetLang("Modal_Title_ChangeUsername");

        $("#modalTitle").text(modalTitleChangeUsername);
        $("#modal_contents").empty();

        $("#modal_contents").append(`<div>
		        <span class="modal-text">${modalChangeUsername}.</span>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="NewAccountName" style="width: 100%;padding: 8px;" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('change_username').click();">
	        </div>
	        <div class="settingsCol inputAndButton">
				${extraButtons}
		        <button class="modalOK" type="button" id="change_username" onclick="Modal_FinaliseAccNameChange()"><span>${
            modalChangeUsernameType}</span></button>
	        </div>`);
        input = document.getElementById("NewAccountName");
    } else if (modaltype.startsWith("find:")) {
        // USAGE: "find:<Program_name>:<Program_exe>:<SettingsFile>" -- example: "find:Steam:Steam.exe:SteamSettings"
        platform = modaltype.split(":")[1].replaceAll("_", " ");
        var platformExe = modaltype.split(":")[2];
        var platformSettingsPath = modaltype.split(":")[3];
        Modal_RequestedLocated(false);

        // Sub in info if this is a basic page
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCurrentBasicPlatform", platform).then((r) => {
            platform = r;
        });
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCurrentBasicPlatformExe", platform).then((r) => {
            platformExe = platform === r ? platformExe : r;
        });

        const modalEnterDirectory = await GetLangSub("Modal_EnterDirectory", { platform: platform }),
            modalLocatePlatform = await GetLangSub("Modal_LocatePlatform", { platformExe: platformExe }),
            modalLocatePlatformFolder = await GetLangSub("Modal_LocatePlatformFolder", { platform: platform }),
            modalLocatePlatformTitle = await GetLangSub("Modal_Title_LocatePlatform", { platform: platform });

        $("#modalTitle").text(modalLocatePlatformTitle);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div style="width: 80vw;">
		        <span class="modal-text">${modalEnterDirectory}</span>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="FolderLocation" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('select_location').click();">
		        <button type="button" id="LocateProgramExe" onclick="window.location = window.location + '?selectFolder=${platformExe}';"><span>${modalLocatePlatform}</span></button>
	        </div>
	        <div class="settingsCol inputAndButton">
		        <div class="folder_indicator notfound"><div id="folder_indicator_text"></div></div>
		        <div class="folder_indicator_bg notfound"><span>${platformExe}</span></div>
		        <button class="modalOK" type="button" id="select_location" onclick="Modal_Finalise('${platform}', '${platformSettingsPath}')"><span>${modalLocatePlatformFolder}</span></button>
	        </div>`);
        input = document.getElementById("FolderLocation");
    } else if (modaltype.startsWith("confirm:")) {
        // USAGE: "confirm:<prompt>
        // GOAL: To return true/false
        let action = modaltype.slice(8);

        const modalConfirmAction = await GetLang("Modal_ConfirmAction");

        let message = "";
        let header = `<h3>${modalConfirmAction}:</h3>`;
        if (action.startsWith("AcceptForgetSteamAcc")) {
            message = await GetLang("Prompt_ForgetSteam");
        } else if (action.startsWith("AcceptForgetDiscordAcc") ||
            action.startsWith("AcceptForgetBasicAcc") ||
            action.startsWith("AcceptForgetOriginAcc") ||
            action.startsWith("AcceptForgetUbisoftAcc") ||
            action.startsWith("AcceptForgetBattleNetAcc") ||
            action.startsWith("AcceptForgetRiotAcc")) {
            message = await GetLangSub("Prompt_ForgetAccount", { platform: getCurrentPage() });
        } else {
            message = `<p>${modaltype.split(":")[2].replaceAll("_", " ")}</p>`;
            // The only exception to confirm:<prompt> was AcceptForgetSteamAcc, as that was confirm:AcceptForgetSteamAcc:steamId
            // Could be more in the future.
            action = action.split(":")[0];
        }

        const modalConfirmActionTitle = await GetLang("Modal_Title_ConfirmAction"),
            yes = await GetLang("Yes"),
            no = await GetLang("No");

        $("#modalTitle").text(modalConfirmActionTitle);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div class="infoWindow">
        <div class="fullWidthContent">${header + message}
            <div class="YesNo">
		        <button type="button" id="modal_true" onclick="Modal_Confirm('${action}', true)"><span>${yes
            }</span></button>
		        <button type="button" id="modal_false" onclick="Modal_Confirm('${action}', false)"><span>${no
            }</span></button>
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

        const modalConfirmAction = await GetLang("Modal_ConfirmAction"),
            modalConfirmActionTitle = await GetLang("Modal_Title_ConfirmAction"),
            ok = await GetLang("Ok");

        let message = "";
        let header = `<h3>${modalConfirmAction}:</h3>`;
        if (action.startsWith("RestartAsAdmin")) {
            message = await GetLang("Prompt_RestartAsAdmin");
            action = (args !== "" ? `location = 'RESTART_AS_ADMIN?arg=${args}'` : "location = 'RESTART_AS_ADMIN'");
        } else {
            message = `<p>${modaltype.split(":")[2].replaceAll("_", " ")}</p>`;
            action = action.split(":")[0];
        }

        $("#modalTitle").text(modalConfirmActionTitle);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div class="infoWindow">
        <div class="fullWidthContent">${header + message}
            <div class="YesNo">
		        <button type="button" id="modal_true" onclick="${action}"><span>${ok}</span></button>
            </div>
        </div>
        </div>`);
        input = document.getElementById("modal_true");
    } else if (modaltype === "accString") {
        platform = getCurrentPage();
        let extraButtons = "";
        if (platform === "Discord") {
            extraButtons = `
                <button class="modalOK extra" type="button" id="set_account_name" onclick="discordCopyJS()"><span><svg viewBox="0 0 448 512" draggable="false" alt="C" class="footerIcoInline"><use href="img/fontawesome/copy.svg#img"></use></svg></span></button>
				<button class="modalOK extra" type="button" id="set_account_name" onclick="OpenLinkInBrowser('https://github.com/TcNobo/TcNo-Acc-Switcher/wiki/Platform:-Discord#adding-accounts-to-the-discord-switcher-list');"><span><svg viewBox="0 0 384 512" draggable="false" alt="C" class="footerIcoInline"><use href="img/fontawesome/question.svg#img"></use></svg></span></button>`;
        }
        Modal_RequestedLocated(false);
        // Sub in info if this is a basic page
        var redirectLink = platform;
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCurrentBasicPlatform", platform).then((r) => {
            platform = r;
        });

        const modalTitleAddNew = await GetLangSub("Modal_Title_AddNew", { platform: platform }),
            modalAddNew = await GetLangSub("Modal_AddNew", { platform: platform }),
            modalAddCurrentAccount = await GetLangSub("Modal_AddCurrentAccount", { platform: platform });

        $("#modalTitle").text(modalTitleAddNew);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div>
		        <span class="modal-text">${modalAddNew}</span>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="CurrentAccountName" style="width: 100%;padding: 8px;" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('set_account_name').click();" onkeypress='return /[^<>:\\.\\"\\/\\\\|?*]/i.test(event.key)'>
	        </div>
	        <div class="settingsCol inputAndButton">
				${extraButtons}
		        <button class="modalOK" type="button" id="set_account_name" onclick="Modal_FinaliseAccString('${
            redirectLink}')"><span>${modalAddCurrentAccount}</span></button>
	        </div>`);
        input = document.getElementById("CurrentAccountName");
    } else if (modaltype === "SetBackground") {
        const modalTitleBackground = await GetLang("Modal_Title_Background"),
            modalSetBackground = await GetLang("Modal_SetBackground"),
            modalChooseLocal = await GetLang("Modal_SetBackground_ChooseImage"),
            modalSetBackgroundButton = await GetLang("Modal_SetBackground_Button");

        $("#modalTitle").text(modalTitleBackground);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div>
		        <p class="modal-text">${modalSetBackground}</p>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="FolderLocation" style="width: 100%;padding: 8px;" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('set_background').click();"'>
	        </div>
	        <div class="settingsCol inputAndButton">
		        <button class="modalOK" type="button" id="set_background" onclick="window.location = window.location + '?selectFile=*.*';"><span>${modalChooseLocal}</span></button>
		        <button class="modalOK" type="button" id="set_background" onclick="Modal_FinaliseBackground()"><span>${modalSetBackgroundButton}</span></button>
	        </div>`);
    } else if (modaltype === "password") {
        let x = await showPasswordModal();
        if (!x) return;
    } else {

        const notice = await GetLang("Notice");

        $("#modalTitle").text(notice);
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

async function showPasswordModal() {
    platform = getCurrentPage();
    // Remove click-off action:
    $(".modalBG")[0].onclick = () => false;
    $("#btnClose-modal")[0].removeAttribute("onclick");
    $("#btnClose-modal")[0].onclick = () => btnBack_Click();

    // Check if a password is set
    const modalNewPassword = await GetLangSub("Modal_EnterNewPassword", { platform: platform }),
        modalAddPassword = await GetLangSub("Modal_EnterPassword", { platform: platform });

    let skipEntry = false;
    let infoText = modalNewPassword;
    let creatingPass = true;
    let dPromise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GiCheckPlatformPassword`, platform).then((r) => {
        if (r === 1) {
            infoText = modalAddPassword;
            creatingPass = false;
        } else if (r === 2) {
	        skipEntry = true; // Skip as it's already entered.
        }
    });
    _ = await dPromise;

    if (skipEntry) return false;

    const modalAddNewTitle = await GetLangSub("Modal_Title_AddNew", { platform: platform }),
        modalEnterPassword = await GetLang("Modal_EnterPasswordShort"),
        modalEnterPasswordRepeat = await GetLang("Modal_EnterPasswordRepeat"),
        modalPasswordsMatch = await GetLang("Modal_PasswordsMatch"),
        modalPasswordsNoMatch = await GetLang("Modal_PasswordsNoMatch"),
        ok = await GetLang("Ok");

	$("#modalTitle").text(modalAddNewTitle);
    $("#modal_contents").empty();
    $("#modal_contents").append(`<div>
		        <span class="modal-text">${infoText}</span>
	        </div>
	        <div class="inputWithTitle">
				<span class="modal-text">${modalEnterPassword}:</span>
		        <input type="password" id="Password" style="width: 100%;padding: 8px;"` + (creatingPass ? "" : `onkeydown="javascript: if(event.keyCode == 13) document.getElementById('set_account_name').click();">`) + `
	        </div>		        ` +
        (creatingPass ? `<div class="inputWithTitle"><span class="modal-text">${modalEnterPasswordRepeat}:</span><input type="password" id="PasswordConfirm" style="width: 100%;padding: 8px;" onkeyup="javascript: if($('#Password').val() !== $('#PasswordConfirm').val()) $('#formNotice').html('${modalPasswordsNoMatch}').css('color','red'); else $('#formNotice').html('${modalPasswordsMatch}').css('color','lime')"></div>	` : "")
	    + `
			<p id="formNotice"></p>
	        <div class="settingsCol inputAndButton">
				<button class="modalOK extra" type="button" id="help" onclick="OpenLinkInBrowser('https://github.com/TcNobo/TcNo-Acc-Switcher/wiki/Platform:-Discord#why-a-password');"><span><svg viewBox="0 0 384 512" draggable="false" alt="C" class="footerIcoInline"><use href="img/fontawesome/question.svg#img"></use></svg></span></button>
		        <button class="modalOK" style="padding: 0 40px;" type="button" id="set_account_name" onclick="Modal_HandlePassword()"><span>${ok}</span></button>
	        </div>`);

    return true;
}

function Modal_SetFilepath(path) {
    $("#FolderLocation").val(path);
}
// For finding files with modal:
function Modal_RequestedLocated(found) {
    try {
        $(".folder_indicator").removeClass("notfound found");
        $(".folder_indicator_bg").removeClass("notfound found");
        if (found === true) {
            $(".folder_indicator").addClass("found");
            $(".folder_indicator_bg").addClass("found");
        } else {
            $(".folder_indicator").addClass("notfound");
            $(".folder_indicator_bg").addClass("notfound");
        }
    } catch (_) {

    }
}

//function Modal_Finalise(platform, platformSettingsPath) {
//    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiUpdatePath", platformSettingsPath, $("#FolderLocation").val());
//    $(".modalBG").fadeOut();
//    window.location.assign(platformSettingsPath.split("Settings")[0]);
//}

async function Modal_Finalise(platform, platformSettingsPath) {
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
    // Supported: Discord, Origin, Riot, BASIC
    let raw = $("#CurrentAccountName").val();
    let name = (raw.indexOf("TCNO:") === -1 ? raw.replace(/[<>: \.\"\/\\|?*]/g, "-") : raw); // Clean string if not a command string.
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", platform + "AddCurrent", name);
    $(".modalBG").fadeOut();
    $("#acc_list").click();
}

function Modal_FinaliseBackground() {
    let pathOrUrl = $("#FolderLocation").val();
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SetBackground", pathOrUrl);
    $(".modalBG").fadeOut();
}

function Modal_FinaliseAccNameChange() {
    let raw = $("#NewAccountName").val();
	let name = (raw.indexOf("TCNO:") === -1 ? raw.replace(/[<>: \.\"\/\\|?*]/g, "-") : raw); // Clean string if not a command string.
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "ChangeUsername", $(".acc:checked").attr("id"), name, getCurrentPage());
}


async function Modal_HandlePassword() {
	if ($("#PasswordConfirm").length && $("#PasswordConfirm").val() !== $("#Password").val())
		return false;
    let pass = $('#Password').val();

    const modalPasswordsNoMatch = await GetLang("Modal_PasswordsNoMatch"),
        toastRetryOrDeleteDiscordCache = await GetLang("Toast_RetryOrDeleteDiscordCache");

    var promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiVerifyPlatformPassword", getCurrentPage(), pass).then((r) => {
        if (!r)
	        window.notification.new({
		        type: "error",
                title: modalPasswordsNoMatch,
                message: toastRetryOrDeleteDiscordCache,
		        renderTo: "toastarea",
		        duration: 5000
	        });
        else {
            $(".modalBG").fadeOut();
            $("#acc_list").click();
        }
    });
    var result = await promise;
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

async function discordCopyJS() {
    // Clicks the User Settings button
    // Then immediately copies the 'src' of the profile image, and the username, as well as the #.
    // Then copy to clipboard.
    const findOptionsButtonText = await GetLang("Prompt_DiscordCopy1"),
        checkStreamerModeText = await GetLang("Prompt_DiscordCopy2"),
        userCheckStreamerMode = await GetLang("Prompt_DiscordCopy3")
		getNameText = await GetLang("Prompt_DiscordCopy4"),
        getAvatarText = await GetLang("Prompt_DiscordCopy5"),
        copyAvatarText = await GetLang("Prompt_DiscordCopy6"),
        closeOptionsText = await GetLang("Prompt_DiscordCopy7"),
        notifyUserText = await GetLang("Prompt_DiscordCopy8"),
        successfullyCopiedText = await GetLang("Prompt_DiscordCopy9"),
        instrutionsText = await GetLang("Prompt_DiscordCopy10"),
        toastCopied = await GetLang("Toast_Copied"),
        toastPasteDiscordConsole = await GetLang("Toast_PasteDiscordConsole");

    let code = `
// ${findOptionsButtonText}
let btns = document.getElementsByClassName("button-14-BFJ");
btns[btns.length-1].click();

// ${checkStreamerModeText}
let streamerMode = false;
try { streamerMode = $("[class^='streamerModeEnabledBtn']") !== null;} catch (e) {streamerMode = false;}
if (streamerMode){
console.log.apply(console, ["%cTcNo Account Switcher%c: ERROR!\\n${userCheckStreamerMode}\\n%chttps://github.com/TcNobo/TcNo-Acc-Switcher/wiki/Platform:-Discord#adding-accounts-to-the-discord-switcher-list ", 'background: #290000; color: #F00','background: #290000; color: white','background: #222; color: lightblue']);
}else{
  // ${getNameText}
  let name = $("[class^='usernameInnerRow']").firstElementChild.innerText + $("[class^='usernameInnerRow']").lastElementChild.innerText;
  // ${getAvatarText}
  let avatar = $("[class^='accountProfileCard']").getElementsByTagName("img")[0].src;

  // ${copyAvatarText}
  copy(\`TCNO: \${avatar}|\${name}\`);

  let possibleExit = $("[class^='contentRegionScroller']").getElementsByTagName('svg');
  let closeButton = possibleExit[possibleExit.length-1].parentElement;

  await new Promise(resolve => setTimeout(resolve, 500)); // Wait 500ms

  // ${closeOptionsText}
  closeButton.click();

  // ${notifyUserText}
  console.log.apply(console, ["%cTcNo Account Switcher%c: ${successfullyCopiedText}\\n${instrutionsText}\\n%chttps://github.com/TcNobo/TcNo-Acc-Switcher/wiki/Platform:-Discord#adding-accounts-to-the-discord-switcher-list ", 'background: #222; color: #bada55','background: #222; color: white','background: #222; color: lightblue']);
}`;

    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "CopyToClipboard", code);
    window.notification.new({
	    type: "success",
	    title: toastCopied,
        message: toastPasteDiscordConsole,
	    renderTo: "toastarea",
	    duration: 5000
    });
}