var currentVersion = "";

// Returns "Steam" or "Riot" for example, based on the current URL
function getCurrentPage() {
    return (window.location.pathname.split("/")[0] !== "" ?
        window.location.pathname.split("/")[0] :
        window.location.pathname.split("/")[1]);
}

async function getCurrentPageFullname() {
    // If a name for text is required, rather than code (Steam instead of steam or basic)
    var platform = getCurrentPage();
    if (platform === "Basic") {
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCurrentBasicPlatform", platform).then((r) => {
            platform = r;
        });
    }
    return platform;
}

window.addEventListener('load',
    () => {
        initTooltips();
    });

function initTooltips() {
    // I don't know of an easier way to do this.
    setTimeout(() => $('[data-toggle="tooltip"]').tooltip(), 1000);
    setTimeout(() => $('[data-toggle="tooltip"]').tooltip(), 2000);
    setTimeout(() => $('[data-toggle="tooltip"]').tooltip(), 4000);
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
    const promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `Get${getCurrentPage()}ForgetAcc`).then((r) => {
        if (!r) showModal(`confirm:AcceptForget${getCurrentPage()}Acc:${reqId}`);
        else Modal_Confirm(`AcceptForget${getCurrentPage()}Acc:${reqId}`, true);
    });
    _ = await promise;
}

// Show the Notes modal for selected account
async function showNotes(e) {
    e.preventDefault();
    showModal(`notes:${$(selectedElem).attr("id")}`);
}

// Get and return note text for the requested account
async function getAccNotes(accId) {
    var accNotes = "";
    const promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `Get${getCurrentPage()}Notes`, accId).then((r) => {
        accNotes = r;
    });
    _ = await promise;
    return accNotes;
}

// STOP IGNORING BATTLENET ACCOUNTS
async function restoreBattleNetAccounts() {
	const toastFailedRestore = await GetLang("Toast_FailedRestore"),
		toastRestored = await GetLang("Toast_Restored");
    const reqBattleNetId = $("#IgnoredAccounts").children("option:selected").toArray().map((item) => {
        return $(item).attr("value");
    });

    const promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "BattleNet_RestoreSelected", reqBattleNetId).then((r) => {
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
    await promise;
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
        const steamId64 = $(selectedElem).attr("id");
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
    if (e !== undefined && e !== null) e.preventDefault();
    if (!getSelected()) return;


    if (request === -1) DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `SwapTo${getCurrentPage()}`, selected.attr("id")); // -1 is for undefined.
    else DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `SwapTo${getCurrentPage()}WithReq`, selected.attr("id"), request);
}

// Copies a game's folder from one userdata directory to another
function CopySettingsFrom(e, game) {
    if (e !== undefined && e !== null) e.preventDefault();
    if (!getSelected()) return;
    if (!game) return;
    const steamId64 = selected.attr("id");
    console.log("Copying settings");
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "CopySettingsFrom", steamId64, game);
}

// Restores a game's userdata folder from 'backup'
function RestoreSettingsTo(e, game) {
    if (e !== undefined && e !== null) e.preventDefault();
    if (!getSelected()) return;
    if (!game) return;
    const steamId64 = selected.attr("id");
    console.log("Restoring settings from backup");
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "RestoreSettingsTo", steamId64, game);
}

//  Manually backup a game's userdata folder
function BackupGameData(e, game) {
    if (e !== undefined && e !== null) e.preventDefault();
    if (!getSelected()) return;
    if (!game) return;
    const steamId64 = selected.attr("id");
    console.log("Backing up settings");
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "BackupGameData", steamId64, game);
}

// Swapping accounts
async function changeImage(e) {
    if (e !== undefined && e !== null) e.preventDefault();
    if (!getSelected()) return;

    const path = $(".acc:checked").next("label").children("img")[0].getAttribute("src").split('?')[0];

    const modalTitleBackground = await GetLang("Modal_Title_Userdata"),
        modalHeading = await GetLang("Modal_SetImageHeader"),
        modalSetButton = await GetLang("Modal_SetImage");

    $("#modalTitle").text(modalTitleBackground);
    $("#modal_contents").empty();
    $("#modal_contents").append(`<div>
		        <p class="modal-text">${modalHeading}</p>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="FolderLocation" autocomplete="off" style="width: 100%;padding: 8px;" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('set_background').click();"'>
	        </div>
	        <div class="settingsCol inputAndButton">
                <button class="modalOK" type="button" id="set_background" onclick="Modal_FinalizeImage('${encodeURI(path)}')"><span>${modalSetButton}</span></button>
	        </div>
            <div class="pathPicker">
                ${await getLogicalDrives()}
            </div>`);

    pathPickerRequestedFile = "AnyFile";
    const input = document.getElementById("FolderLocation");
    $(".pathPicker").on("click", pathPickerClick);
    $(".modalBG").fadeIn(() => {
        try {
            if (input === undefined) return;
            input.focus();
            input.select();
        }
        catch (err) {

        }
    });
}

// Open Game Stats menu 1: Enable/Disable stats for specific games
async function ShowGameStatsSetup(e) {
    if (e !== undefined && e !== null) e.preventDefault();
    if (!getSelected()) return;

    const accountId = selected.attr("id");

    const modalTitle = await GetLangSub("Modal_Title_GameStats", { accountName: selected.attr("displayname") }),
        modalHeading = await GetLang("Modal_GameStats_Header");

    const currentPage = await getCurrentPageFullname();
    let enabledGames = [],
        disabledGames = [],
        safeGameNames = [];

    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GetEnabledGames", currentPage, accountId).then((r) => {
        enabledGames = r;
    });
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GetDisabledGames", currentPage, accountId).then((r) => {
        disabledGames = r;
    });

    $("#modalTitle").text(modalTitle);
    $("#modal_contents").empty();
    let html = "";
    html += `<div class="gameStatsWindow">
		        <p>${modalHeading}</p>
            <div class="rowSetting">`;

    for (const x in enabledGames) {
        let game = enabledGames[x];
        let safeGame = game.replace(/\s/g, '');
        html += `<div class="form-check mb-2"><input class="form-check-input" type="checkbox" id="${safeGame
            }" checked><label class="form-check-label" for="${safeGame
            }"></label><label for="${safeGame}">${game}<br></label></div>`;
        safeGameNames.push(safeGame);
    }
    for (const x in disabledGames) {
        let game = disabledGames[x];
        let safeGame = game.replace(/\s/g, '');
        html += `<div class="form-check mb-2"><input class="form-check-input" type="checkbox" id="${safeGame
            }"><label class="form-check-label" for="${safeGame
            }"></label><label for="${safeGame}">${game}<br></label></div>`;
        safeGameNames.push(safeGame);
    }

                //@foreach (var item in AData.EnabledPlatformSorted())
                //{
                //
                //}
    html += "</div></div>";
    $("#modal_contents").append(html);

    for (const x in safeGameNames) {
        let game = safeGameNames[x];
        $(`#${game}`).change(function () {
            toggleGameStats(game, this.checked);
        });
    }

    $(".modalBG").fadeIn(() => {
        try {
            if (input === undefined) return;
            input.focus();
            input.select();
        }
        catch (err) {

        }
    });
    // Later:
    // On enabling game, required variables are collected from user, and game stats activated for that game.
    // User stats also collected and displayed.
    // On disabling game, stats are cleared for said game.
}

async function toggleGameStats(safeGame, isChecked) {
    if (!getSelected()) return;
    const accountId = selected.attr("id");
    const game = $(`label[for='${safeGame}']:last`).text();
    console.log(game, isChecked);

    if (!isChecked) {
        // Unchecked: Remove entry and continue.
        DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `DisableGame`, game, accountId);
        return;
    }

    // Checked: Get required variables and present to user.
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GetRequiredVars`, game).then((r) => {
        showGameVarCollectionModel(game, r);
    });
}

async function showGameVarCollectionModel(game, requiredVars) {
    if (!getSelected()) return;
    const accountId = selected.attr("id");
    const currentPage = await getCurrentPageFullname();

    const modalTitle = await GetLangSub("Modal_Title_GameVars", { game: game }),
        modalHeading = await GetLangSub("Modal_GameVars_Header", { game: game, username: selected.attr("displayname"), platform: currentPage }),
        submit = await GetLang("Submit");

    $("#modalTitle").text(modalTitle);
    $("#modal_contents").empty();
    let html = "";
    html += `<div class="gameStatsWindow">
                <p>${modalHeading}</p>
            <div class="rowSetting">`;

    for (const [key, value] of Object.entries(requiredVars)) {
        console.log(key, value);
        html +=
            `<div class="form-text"><span>${value}</span><input type="text" id="acc${key}" spellcheck="false" placeholder="${key}"></div>`;
    }

    html += `</div><div class="settingsCol inputAndButton">
        <button class="modalOK" type="button" id="set_password" onclick="Modal_FinaliseGameVars('${game}', '${accountId}')"><span>${
        submit}</span></button></div>`;
    $("#modal_contents").append(html);

    $(".modalBG").fadeIn(() => {
        try {
            if (input === undefined) return;
            input.focus();
            input.select();
        }
        catch (err) {

        }
    });
}

async function Modal_FinaliseGameVars(game, accountId) {
    console.log(game, accountId);

    // Get list of variable keys
    var requiredVars = [];
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GetRequiredVars`, game).then((r) => {
        requiredVars = r;
        console.log(requiredVars, typeof(requiredVars));
    });

    // Get value for each key and create dictionary
    let returnDict = {};
    for (const [key, value] of Object.entries(requiredVars)) {
        console.log(key, value, $(`#acc${key}`).val());
        returnDict[key] = $(`#acc${key}`).val();
    }

    // Add user statistics for game, with collected variables
    $(".modalBG").fadeOut();
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `SetGameVars`, game, accountId, returnDict);

    location.reload();
}


async function Modal_FinalizeImage(dest) {
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `ImportNewImage`, JSON.stringify({ dest: dest, path: $("#FolderLocation").val() }));
}

// Open Steam\userdata\<steamID> folder
function openUserdata(e) {
    if (e !== undefined && e !== null) e.preventDefault();
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
    const promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiExportAccountList", selectedElem).then((r) => {
        const filename = r.split('/');
        saveFile(filename[filename.length - 1], r);
        exportingAccounts = false;
    });
    _ = await promise;
}

function saveFile(fileName, urlFile) {
	const a = document.createElement("a");
	a.style = "display: none";
	document.body.appendChild(a);
	a.href = urlFile;
	a.download = fileName;
	a.click();
	a.remove();
}



$(".acc").dblclick(() => {
    alert("Handler for .dblclick() called.");
    swapTo();
});
// Link handling
function OpenLinkInBrowser(link) {
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "OpenLinkInBrowser", link);
}

let pathPickerRequestedFile = "";

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
                        <a onclick="OpenLinkInBrowser('https://github.com/TcNobo/TcNo-Acc-Switcher');return false;" href=""><svg viewBox="0 0 24 24" draggable="false" alt="GitHub" class="modalIcoGitHub"><use href="img/icons/ico_github.svg#icoGitHub"></use></svg>${
                modalInfoViewGitHub}</a>
                        <a onclick="OpenLinkInBrowser('https://s.tcno.co/AccSwitcherDiscord');return false;" href=""><svg viewBox="0 0 24 24" draggable="false" alt="Discord" class="modalIcoDiscord"><use href="img/icons/ico_discord.svg#icoDiscord"></use></svg>${
                modalInfoBugReport}</a>
                        <a onclick="OpenLinkInBrowser('https://tcno.co');return false;" href=""><svg viewBox="0 0 24 24" draggable="false" alt="Website" class="modalIcoNetworking"><use href="img/icons/ico_networking.svg#icoNetworking"></use></svg>${
                modalInfoVisitSite}</a>
                        <a onclick="OpenLinkInBrowser('https://github.com/TcNobo/TcNo-Acc-Switcher/blob/master/DISCLAIMER.md');return false;" href=""><svg viewBox="0 0 2084 2084" draggable="false" alt="GitHub" class="modalIcoDoc"><use href="img/icons/ico_doc.svg#icoDoc"></use></svg>${
                modalInfoDisclaimer}</a>
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
                <h2>${modalCrowdinHeader}<i class="fas fa-heart heart"></i></h2>
                    <p>${modalCrowdinInfo}</p>
                    <ul>${listUsers}</ul>
            </div></div>`);
    } else if (modaltype.startsWith("changeUsername")) {
        // USAGE: "changeUsername"
        Modal_RequestedLocated(false);
        var platformName = getCurrentPage();
        let extraButtons = "";
        if (platformName === "Basic") {
            await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "PlatformUserModalExtraButtons").then((r) => {
                extraButtons = r;
            });
            platformName = await getCurrentPageFullname();
        }

        const modalChangeUsername =
                  await GetLangSub("Modal_ChangeUsername", { platform: platformName }),
            modalChangeUsernameType = await GetLangSub("Modal_ChangeUsernameType", { platform: platformName }),
            modalTitleChangeUsername = await GetLang("Modal_Title_ChangeUsername");

        $("#modalTitle").text(modalTitleChangeUsername);
        $("#modal_contents").empty();

        $("#modal_contents").append(`<div>
		        <span class="modal-text">${modalChangeUsername}.</span>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="NewAccountName" style="width: 100%;padding: 8px;" autocomplete="off" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('change_username').click();">
	        </div>
	        <div class="settingsCol inputAndButton">
				${extraButtons}
		        <button class="modalOK" type="button" id="change_username" onclick="Modal_FinaliseAccNameChange()"><span>${
            modalChangeUsernameType}</span></button>
	        </div>`);
        input = document.getElementById("NewAccountName");
    } else if (modaltype.startsWith("setAppPassword")) {
        // USAGE: "changeUsername"
        const modalChangeUsername = await GetLang("Modal_SetPassword"),
            modalSetPasswordButton = await GetLang("Modal_SetPassword_Button"),
            modalTitleSetPassword = await GetLang("Modal_Title_SetPassword"),
            modalSetPasswordInfo = await GetLangSub("Modal_SetPassword_Info", { link: "https://github.com/TcNobo/TcNo-Acc-Switcher/wiki/FAQ---More-Info#can-i-put-this-program-on-a-usb-portable" });

        $("#modalTitle").text(modalTitleSetPassword);
        $("#modal_contents").empty();

        $("#modal_contents").append(`<h3>${modalChangeUsername}</h3>
	        <div class="inputAndButton">
		        <input type="text" id="SwitcherPassword" style="width: 100%;padding: 8px;" autocomplete="off" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('set_password').click();">
	        </div>
            <div>
                <span class="modal-text">${modalSetPasswordInfo}</span>
            </div>
	        <div class="settingsCol inputAndButton">
		        <button class="modalOK" type="button" id="set_password" onclick="Modal_FinaliseSwitcherPassword()"><span>${
            modalSetPasswordButton}</span></button>
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
            modalLocatePlatformFolder = await GetLangSub("Modal_LocatePlatformFolder", { platform: platform }),
            modalLocatePlatformTitle = await GetLangSub("Modal_Title_LocatePlatform", { platform: platform });

        $("#modalTitle").text(modalLocatePlatformTitle);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div style="width: 80vw;">
		        <span class="modal-text">${modalEnterDirectory}</span>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="FolderLocation" oninput="updateIndicator('')" autocomplete="off" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('select_location').click();">
	        </div>
	        <div class="settingsCol inputAndButton">
		        <div class="folder_indicator notfound"><div id="folder_indicator_text"></div></div>
		        <div class="folder_indicator_bg notfound"><span>${platformExe}</span></div>
		        <button class="modalOK" type="button" id="select_location" onclick="Modal_Finalise('${platform}', '${
            platformSettingsPath}')"><span>${modalLocatePlatformFolder}</span></button>
	        </div>
            <div class="pathPicker">
                ${await getLogicalDrives()}
            </div>`);

        pathPickerRequestedFile = platformExe;
        $(".pathPicker").on("click", pathPickerClick);
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
        } else if (action.startsWith("AcceptForgetBasicAcc") ||
            action.startsWith("AcceptForgetBattleNetAcc")) {
            message = await GetLangSub("Prompt_ForgetAccount", { platform: await getCurrentPageFullname() });
        } else if (action.startsWith("ClearStats")) {
            message = await GetLang("Prompt_ClearStats");
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
		        <button type="button" id="modal_true" onclick="Modal_Confirm('${action}', true)"><span>${yes}</span></button>
		        <button type="button" id="modal_false" onclick="Modal_Confirm('${action}', false)"><span>${no}</span></button>
            </div>
        </div>
        </div>`);
    } else if (modaltype.startsWith("notes:")) {
        // USAGE: "notes:accId"
        // GOAL: Display previously set accNotes, and upon SAVE click, save the new notes.
        let accId = modaltype.slice(6);
        if (!getSelected()) return;

        $("#modalTitle").text(await GetLangSub("Modal_Title_AccountNotes", { accountName: selected.attr("displayname") }));
        const save = await GetLang("Save"),
            cancel = await GetLang("Button_Cancel");

        var notes = await getAccNotes(accId);

        $("#modal_contents").empty();
        $("#modal_contents").append(`<div class="infoWindow">
        <div class="fullWidthContent">
            <div class="accNotesContainer">
                <textarea id="accNotes">${notes}</textarea>
            </div>
            <div class="YesNo">
		        <button type="button" id="modal_true" onclick="Modal_SaveNotes('${accId}')"><span>${save}</span></button>
		        <button type="button" id="modal_false" onclick="$('.modalBG').fadeOut();"><span>${cancel}</span></button>
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
            action = (args !== "" ? `restartAsAdmin(${args})` : "restartAsAdmin()");
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
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "PlatformUserModalExtraButtons").then((r) => {
            extraButtons = r;
        });

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
		        <input type="text" id="CurrentAccountName" autocomplete="off" style="width: 100%;padding: 8px;" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('set_account_name').click();" onkeypress='return /[^<>:\\.\\"\\/\\\\|?*]/i.test(event.key)'>
	        </div>
	        <div class="settingsCol inputAndButton">
				${extraButtons}
		        <button class="modalOK" type="button" id="set_account_name" onclick="Modal_FinaliseAccString('${
            redirectLink}')"><span>${modalAddCurrentAccount}</span></button>
	        </div>`);
        input = document.getElementById("CurrentAccountName");
    } else if (modaltype === "SetBackground") {
        const modalTitleBackground = await GetLang("Modal_Title_Background"),
            modalHeading = await GetLang("Modal_SetBackground"),
            modalSetButton = await GetLang("Modal_SetBackground_Button");

        $("#modalTitle").text(modalTitleBackground);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div>
		        <p class="modal-text">${modalHeading}</p>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="FolderLocation" oninput="updateIndicator('')" autocomplete="off" style="width: 100%;padding: 8px;" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('set_background').click();"'>
	        </div>
	        <div class="settingsCol inputAndButton">
		        <button class="modalOK" type="button" id="set_background" onclick="Modal_FinaliseBackground()"><span>${
            modalSetButton}</span></button>
	        </div>
            <div class="pathPicker">
                ${await getLogicalDrives()}
            </div>`);

        pathPickerRequestedFile = "AnyFile";
        $(".pathPicker").on("click", pathPickerClick);
        input = document.getElementById("FolderLocation");
    } else if (modaltype === "SetUserdata") {
        const modalTitleBackground = await GetLang("Modal_Title_Userdata"),
            modalHeading = await GetLang("Modal_SetUserdata"),
            modalSetButton = await GetLang("Modal_SetUserdata_Button");

        $("#modalTitle").text(modalTitleBackground);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div>
		        <p class="modal-text">${modalHeading}</p>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="FolderLocation" oninput="updateIndicator('')" autocomplete="off" style="width: 100%;padding: 8px;" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('set_background').click();"'>
	        </div>
	        <div class="settingsCol inputAndButton">
                <button class="modalOK" type="button" id="set_background" onclick="Modal_FinaliseUserDataFolder()"><span>${
            modalSetButton}</span></button>
	        </div>
            <div class="pathPicker">
                ${await getLogicalDrives()}
            </div>`);

        pathPickerRequestedFile = "AnyFolder";
        $(".pathPicker").on("click", pathPickerClick);
        input = document.getElementById("FolderLocation");
    } else if (modaltype === "ShowStats") {
        let modalTitle = await GetLang("Modal_Title_Stats"),
            modalText = "";

        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GetStatsString").then((r) => {
            modalText = r;
        });

        $("#modalTitle").text(modalTitle);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div class="infoWindow">
        <div class="fullWidthContent">
            <div style="text-align:left"><p>${modalText}</p></div>
        </div>
        </div>`);
    } else {

        const notice = await GetLang("Notice");

        $("#modalTitle").text(notice);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div class="infoWindow"><div class="fullWidthContent">${modaltype}</div></div>`);
    }
    $(".modalBG").fadeIn(() => {
        try {
            if (input === undefined) return;
            input.focus();
            input.select();
        }
        catch (err) {

        }
    });
}

async function getLogicalDrives() {
    var folderContent = "";
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GetLogicalDrives").then((r) => {
        folderContent = "<div>";
        r.Folders.forEach((f) => { folderContent += "<span class=\"folder\" path=\"" + f + "\">" + f + "</span>"; });
        folderContent += "</div>";
    });
    return folderContent;
}

function updateIndicator(e) {
    console.log("Update Indicator REQUESTED");
    // Update Found/Not Found preview
    var foundRequested;
    if (pathPickerRequestedFile === "AnyFolder" && e !== "" && $(e.target).hasClass("folder")) {
        foundRequested = true;
    } else if (pathPickerRequestedFile === "AnyFile" && e !== "" && !$(e.target).hasClass("folder")) {
        foundRequested = true;
    } else {
        // Everything else
        pathPickerRequestedFile = pathPickerRequestedFile.replace("*", "");
        foundRequested = $("#FolderLocation").val().toLowerCase().includes(pathPickerRequestedFile.toLowerCase());
    }
    Modal_RequestedLocated(foundRequested);
}
async function pathPickerClick(e) {
    const result = $(e.target).attr("path");
    if (result === undefined) return;
    //console.log(result);
    $("#FolderLocation").val(result);
    updateIndicator(e); // Because the above doesn't trigger the event
    const currentSpanPath = $(e.target).attr("path");
    var folderContent = "";

    // Because this is reset: see if has .exe inside it.
    if ($(e.target).hasClass("folder") && pathPickerRequestedFile.endsWith(".exe")) {
        //console.log($(e.target).parent().html());
        Modal_RequestedLocated($(e.target).parent().html().toLowerCase().includes(pathPickerRequestedFile.toLowerCase()));
    }
    // If is not currently open: Continue
    if ($(e.target).hasClass("c")) return;

    $(".pathPicker .c").each((_, s) => {
        var path = $(s).attr("path");
        if (!result.includes(path)) {
            $(s).parent().replaceWith("<span class=\"folder\" path=\"" + path + "\">" + (path.at(-1) !== "\\" ? path.split("\\").at(-1) : path) + "</span>");
        }
    });

    // Reset all selected-path highlights.
    $(".pathPicker .selected-path").removeClass("selected-path");
    $(e.target).addClass("selected-path");

    // Expand folder
    if ($(e.target).hasClass("folder") && !$(e.target).hasClass("c")) {
        let getFunc = "GetFoldersAndFiles";
        if (pathPickerRequestedFile === "AnyFolder") getFunc = "GetFolders";

        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", getFunc, result).then((r) => {
            folderContent = "<div path=\"" + currentSpanPath + "\"><span class=\"folder c head selected-path\" path=\"" + currentSpanPath + "\">" + (currentSpanPath.at(-1) !== "\\" ? currentSpanPath.split("\\").at(-1) : currentSpanPath) + "</span>";
            r.Folders.forEach((f) => {
                folderContent += "<span class=\"folder\" path=\"" + f + "\">" + (f.at(-1) !== "\\" ? f.split("\\").at(-1) : f) + "</span>";
            });
            r.Files.forEach((f) => {
                folderContent += "<span " + (f.includes(pathPickerRequestedFile) ? "class=\"suggested\" " : "") + "path=\"" + f + "\">" + (f.at(-1) !== "\\" ? f.split("\\").at(-1) : f) + "</span>";
            });
            folderContent += "</div>";

            //console.log(folderContent);
            $(e.target).replaceWith(folderContent);
        });

        // After expanding, see if has .exe inside it.
        if ($(e.target).hasClass("folder") && pathPickerRequestedFile.endsWith(".exe")) {
            Modal_RequestedLocated(folderContent.toLowerCase().includes(pathPickerRequestedFile.toLowerCase()));
        }
    }
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
    if (window.location.href.includes("PreviewCss")) {
        // Do nothing for CSS preview page.
        return;
    }

    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiUpdatePath", platformSettingsPath, $("#FolderLocation").val());
    $(".modalBG").fadeOut();

    location.reload();
}

async function Modal_Confirm(action, value) {
    if (window.location.href.includes("PreviewCss")) {
        // Do nothing for CSS preview page.
        return;
    }

    const success = await GetLang("Success");
    const promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiConfirmAction", action, value).then((r) => {
        if (r === "refresh") location.reload();
        else if (r === "success")
            window.notification.new({
                type: "success",
                title: "",
                message: success,
                renderTo: "toastarea",
                duration: 3000
            });
    });
    await promise;
    $(".modalBG").fadeOut();
}

async function Modal_FinaliseAccString(platform) {
    if (window.location.href.includes("PreviewCss")) {
        // Do nothing for CSS preview page.
        return;
    }

    // Supported: BASIC
    const raw = $("#CurrentAccountName").val();
    let name = raw;

    // Clean string if not a command string.
    if (raw.indexOf(":{") === -1) {
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiGetCleanFilePath", raw).then((r) => {
            name = r;
        });
    }

    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", platform + "AddCurrent", name);
    $(".modalBG").fadeOut();
    $("#acc_list").click();
}

function Modal_FinaliseBackground() {
    const pathOrUrl = $("#FolderLocation").val();
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SetBackground", pathOrUrl);
    $(".modalBG").fadeOut();
}

function Modal_FinaliseSwitcherPassword() {
    const switcherPassword = $("#SwitcherPassword").val();
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SetSwitcherPassword", switcherPassword);
    $(".modalBG").fadeOut();
}

function Modal_FinaliseUserDataFolder() {
    if (window.location.href.includes("PreviewCss")) {
        // Do nothing for CSS preview page.
        return;
    }

    const pathOrUrl = $("#FolderLocation").val();
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SetUserData", pathOrUrl);
    $(".modalBG").fadeOut();
}

function Modal_FinaliseAccNameChange() {
    if (window.location.href.includes("PreviewCss")) {
        // Do nothing for CSS preview page.
        return;
    }

    const raw = $("#NewAccountName").val();
	const name = (raw.indexOf("TCNO:") === -1 ? raw.replace(/[<>: \.\"\/\\|?*]/g, "-") : raw); // Clean string if not a command string.
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "ChangeUsername", $(".acc:checked").attr("id"), name, getCurrentPage());
}

async function Modal_SaveNotes(accId) {
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `Set${getCurrentPage()}Notes`, accId, $('#accNotes').val()).then((r) => {
        location.reload(true);
    });
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

async function usernameModalCopyText() {
    const toastTitle = await GetLang("Toast_Copied");
    let toastHintText = "";
    const platform = getCurrentPage();
    let code = "";

    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "PlatformHintText", platform).then((r) => {
        toastHintText = r;
    });
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "PlatformUserModalCopyText").then((r) => {
        code = r;
    });

    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "CopyToClipboard", code);
    window.notification.new({
	    type: "success",
        title: toastTitle,
        message: toastHintText,
	    renderTo: "toastarea",
	    duration: 5000
    });
}


// Basic account switcher: Shortcut dropdown
var sDropdownOpen = false;
var sDropdownInitialized = false;
function sDropdownReposition() {
    const dropDownContainer = $("#shortcutDropdown");
    const btn = $("#shortcutDropdownBtn");
    const dropDownItemsContainer = $(".shortcutDropdownItems")[0];
    const btnPos = btn[0].getBoundingClientRect();
    dropDownContainer.css({ top: btnPos.top - dropDownContainer.height() - btn.height() - 16, left: btnPos.left + 16 - (dropDownContainer.width() / 2) });

    // If overflowing - Widen by scrollbar width to prevent weird overflow gap on side
    if (checkOverflow(dropDownItemsContainer) && dropDownItemsContainer.style.minWidth === '') {
        let scrollbarWidth = (dropDownItemsContainer.offsetWidth - dropDownItemsContainer.clientWidth);
        let hasContextMenu = $(".HasContextMenu");
        if (hasContextMenu.length > 0) {
            let computedStyle = window.getComputedStyle($(".HasContextMenu")[0]);
            let computedStyleContainer = window.getComputedStyle($("#shortcutDropdown")[0]);
            let marginX = parseInt(computedStyle.marginLeft) + parseInt(computedStyle.marginRight);
            let marginY = parseInt(computedStyle.marginBottom) + parseInt(computedStyle.marginTop);
            let paddingX = parseInt(computedStyleContainer.paddingLeft) + parseInt(computedStyleContainer.paddingRight);
            let paddingY = parseInt(computedStyleContainer.paddingTop) + parseInt(computedStyleContainer.paddingBottom);
            let borderY = parseInt(computedStyleContainer.borderTopWidth) + parseInt(computedStyleContainer.borderBottomWidth);
            dropDownContainer.css({
                minWidth: $(dropDownItemsContainer).width() + scrollbarWidth + marginX + paddingX
            });
            $(dropDownItemsContainer).css({
                maxHeight: dropDownContainer.height() - paddingY - borderY + marginY
            });
        }
    }
}

function sDropdownInit() {
    if (sDropdownInitialized) return;
    sDropdownInitialized = true;
    // Create sortable list
    sortable(".shortcuts, #shortcutDropdownItems", {
        connectWith: "shortcutJoined",
        forcePlaceholderSize: true,
        placeholderClass: "shortcutPlaceholder",
        items: ":not(#btnOpenShortcutFolder)"
    });

    $(".shortcuts, #shortcutDropdownItems").toArray().forEach(el => {
// ReSharper disable once Html.EventNotResolved
        el.addEventListener("sortstart", function () {
            $(".shortcuts").addClass("expandShortcuts");
        });
// ReSharper disable once Html.EventNotResolved
        el.addEventListener("sortstop", function () {
            $(".shortcuts").removeClass("expandShortcuts");
            sDropdownReposition();
            serializeShortcuts();
        });
    });
}

// https://stackoverflow.com/a/143889
function checkOverflow(el) {
    var curOverflow = el.style.overflow;

    if (!curOverflow || curOverflow === "visible")
        el.style.overflow = "hidden";

    var isOverflowing = el.clientWidth < el.scrollWidth
        || el.clientHeight < el.scrollHeight;

    el.style.overflow = curOverflow;

    return isOverflowing;
}

function shortcutDropdownBtnClick() {
    if (!sDropdownOpen) {
        sDropdownInit();
        $("#shortcutDropdown").show();
        sDropdownReposition();
        $("#shortcutDropdownBtn").addClass("flip");
        sDropdownOpen = true;
    } else {
        $("#shortcutDropdown").hide();
        $("#shortcutDropdownBtn").removeClass("flip");
        sDropdownOpen = false;
    }
}

function serializeShortcuts() {
    var output = {};
    // Serialize highlighted items
    var numHighlightedShortcuts = $(".shortcuts button").children().length;
    $(".shortcuts button").each((i, e) => output[i - numHighlightedShortcuts] = $(e).attr("id"));

    // Serialize dropdown items
    $(".shortcutDropdownItems button").each((i, e) => {
        if ($(e).attr("id") === "btnOpenShortcutFolder") return;
        output[i] = $(e).attr("id");
    });

    if (getCurrentPage() === "Steam")
        DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SaveShortcutOrderSteam", output);
    else if (getCurrentPage() === "BattleNet")
        DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SaveShortcutOrderBNet", output);
    else
        DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SaveShortcutOrder", output);
}

// Context menu buttons
function shortcut(action) {
    const reqId = $(selectedElem).prop("id");
    console.log(reqId);
    if (getCurrentPage() === "Steam")
        DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "HandleShortcutActionSteam", reqId, action);
    else if (getCurrentPage() === "BattleNet")
        DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "HandleShortcutActionBNet", reqId, action);
    else
        DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "HandleShortcutAction", reqId, action);
    if (action === "hide") $(selectedElem).remove();
}

async function updateBarClick() {
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "UpdateNow");
}

async function initSavingHotKey() {
    hotkeys("ctrl+s", function (event) {
        event.preventDefault();
        DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCtrlS", getCurrentPage());
    });
}

async function initCopyHotKey() {
    const toastCopied = await GetLang("Toast_Copied");
    hotkeys("ctrl+c,ctrl+shift+c,alt+c", function (event, handler) {
        // Doesn't prevent default!
        switch (handler.key) {
        case "ctrl+shift+c":
        case "alt+c":
                copyToClipboard($(selectedElem).prop("id"));
            break;
        case "ctrl+c":
                copyToClipboard($(selectedElem).attr("displayname"));
            break;
        }

        window.notification.new({
            type: "info",
            title: "",
            message: toastCopied,
            renderTo: "toastarea",
            duration: 2000
        });
    });
}

// Figures out the best place for a tooltip and returns that location
// el MUST BE A JS VARIABLE
function getBestOffset(el) {
    // Because this can be placed below, and go off the screen.. Figure out where the element is.
    const parentPos = el[0].getBoundingClientRect();
    const parentWidth = el.width();

    const parentLeft = parentPos.left;
    const parentRight = parentLeft + parentWidth;

    // Because this can be placed right or below, and go off the screen.. Figure out where the element is.
    var bestOffset = "bottom";
    // Too close to sides -- Basically 1 account gap
    if (parentLeft < 100) bestOffset = "right";
    else if (screen.width - parentRight < 100) bestOffset = "left";
    return bestOffset;
}

async function highlightCurrentAccount(curAcc) {
    // Remove existing highlighted elements, if any.
    $(".currentAcc").each((_, e) => {
        var j = $(e);
        j.removeClass("currentAcc");
        j.parent().removeAttr("title").removeAttr("data-original-title").removeAttr("data-placement");
    });

    // Start adding classes
    const tooltip = await GetLang("Tooltip_CurrentAccount");
    const parentEl = $(`[for='${curAcc}']`).addClass("currentAcc").parent();
    parentEl.attr("title", tooltip);
    parentEl.attr("data-placement", getBestOffset(parentEl));

    initTooltips();
}

async function showNoteTooltips() {
    const noteArr = $(".acc_note").toArray();
    if (noteArr.length === 0) return;

    noteArr.forEach((e) => {
        var j = $(e);
        var note = j.text();
        var parentEl = j.parent().parent();
        parentEl.removeAttr("title").removeAttr("data-original-title").removeAttr("data-placement");
        parentEl.attr("title", note);
        parentEl.attr("data-placement", getBestOffset(parentEl));
    }).then(initTooltips());
}

async function restartAsAdmin(args = "") {
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiRestartAsAdmin", args);
}