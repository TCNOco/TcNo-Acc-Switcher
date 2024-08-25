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
        platform = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCurrentBasicPlatform", platform);
    }
    return platform;
}

window.addEventListener('load',
    () => {
        initTooltips();
    });

function initTooltips() {
    // I don't know of an easier way to do this.
    $('[data-toggle="tooltip"]').tooltip();
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

GetLang = async(k) => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiLocale", k);
GetCrowdin = async() => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCrowdinList");
GetLangSub = async(key, obj) => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiLocaleObj", key, obj);

copyToClipboard = async(str) => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "CopyToClipboard", str);

// FORGETTING ACCOUNTS
async function forget(e) {
    e.preventDefault();
    const reqId = $(selectedElem).attr("id");
    const pageForgetAcc = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `Get${getCurrentPage()}ForgetAcc`);
    if (!pageForgetAcc) showModal(`confirm:AcceptForget${getCurrentPage()}Acc:${reqId}`);
    else await Modal_Confirm(`AcceptForget${getCurrentPage()}Acc:${reqId}`, true);
}

// Show the Notes modal for selected account
async function showNotes(e) {
    e.preventDefault();
    showModal(`notes:${$(selectedElem).attr("id")}`);
}

// Get and return note text for the requested account
getAccNotes = async(accId) => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `Get${getCurrentPage()}Notes`, accId);

async function copy(request, e) {
    e.preventDefault();

    // Different function groups based on platform
    switch (getCurrentPage()) {
        case "Steam":
            steam();
            break;
        default:
            await copyToClipboard(unEscapeString($(selectedElem).attr(request)));
            break;
    }
    return;


    // Steam:
    async function steam() {
        const steamId64 = $(selectedElem).attr("id");
        switch (request) {
            case "URL":
                await copyToClipboard(`https://steamcommunity.com/profiles/${steamId64}`);
                break;
            case "SteamId32":
            case "SteamId3":
            case "SteamId":
                await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "CopySteamIdType", request, steamId64);
                break;

                // Links
            case "SteamRep":
                await copyToClipboard(`https://steamrep.com/search?q=${steamId64}`);
                break;
            case "SteamID.uk":
                await copyToClipboard(`https://steamid.uk/profile/${steamId64}`);
                break;
            case "SteamID.io":
                await copyToClipboard(`https://steamid.io/lookup/${steamId64}`);
                break;
            case "SteamIDFinder.com":
                await copyToClipboard(`https://steamidfinder.com/lookup/${steamId64}/`);
                break;
            default:
                await copyToClipboard(unEscapeString($(selectedElem).attr(request)));
        }
    }
}

// Take a string that is HTML escaped, and return a normal string back.
unEscapeString = (s) => s.replace("&lt;", "<").replace("&gt;", ">").replace("&#34;", "\"").replace("&#39;", "'").replace("&#47;", "/");


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
async function swapTo(request, e) {
    if (e !== undefined && e !== null) e.preventDefault();
    if (!getSelected()) return;


    if (request === -1) await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `SwapTo${getCurrentPage()}`, selected.attr("id")); // -1 is for undefined.
    else await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `SwapTo${getCurrentPage()}WithReq`, selected.attr("id"), request);
}

// Copies a game's folder from one userdata directory to another
async function CopySettingsFrom(e, game) {
    if (e !== undefined && e !== null) e.preventDefault();
    if (!getSelected()) return;
    if (!game) return;
    const steamId64 = selected.attr("id");
    console.log("Copying settings");
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "CopySettingsFrom", steamId64, game);
}

// Restores a game's userdata folder from 'backup'
async function RestoreSettingsTo(e, game) {
    if (e !== undefined && e !== null) e.preventDefault();
    if (!getSelected()) return;
    if (!game) return;
    const steamId64 = selected.attr("id");
    console.log("Restoring settings from backup");
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "RestoreSettingsTo", steamId64, game);
}

//  Manually backup a game's userdata folder
async function BackupGameData(e, game) {
    if (e !== undefined && e !== null) e.preventDefault();
    if (!getSelected()) return;
    if (!game) return;
    const steamId64 = selected.attr("id");
    console.log("Backing up settings");
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "BackupGameData", steamId64, game);
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
async function ShowGameStatsSetup(e = null) {
    if (e !== undefined && e !== null) e.preventDefault();
    if (!getSelected()) return;

    const accountId = selected.attr("id");

    const modalHeading = await GetLangSub("Modal_GameStats_Header", { accountName: getDisplayName() }),
        modalTitle= await GetLang("Modal_Title_GameStats"),
        edit = await GetLang("Edit"),
        refresh = await GetLang("Refresh");

    const currentPage = await getCurrentPageFullname();
    const safeGameNames = [];

    const enabledGames =
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GetEnabledGames", currentPage, accountId);
    const disabledGames = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GetDisabledGames", currentPage, accountId);

    $("#modalTitle").text(modalTitle);
    $("#modal_contents").empty();
    let html = "";
    html += `<div class="gameStatsWindow">
		        <p>${modalHeading}</p>
                <div class="modalScrollSection">`;

    for (const x in enabledGames) {
        const game = enabledGames[x];
        const safeGame = game.replace(/[/\\?%*:|"<>\s]/g, "");
        html += `<div class="rowSetting">
                    <div class="form-check mb-2">
                        <input class="form-check-input" type="checkbox" id="${safeGame}" checked><label class="form-check-label" for="${safeGame}"></label><label for="${safeGame}">${game}<br></label></div>
                    <div>
                        <button type="button" onclick="showGameStatsVars('${game}')"><span>${edit}</span></button>
                        <button type="button" onclick="refreshAccount('${game}', '${accountId}')"><span>${refresh}</span></button>
                    </div>
                </div>`;
        safeGameNames.push(safeGame);
    }
    for (const x in disabledGames) {
        const game = disabledGames[x];
        const safeGame = game.replace(/[/\\?%*:|"<>\s]/g, "");
        html += `   <div class="form-check mb-2">
                        <input class="form-check-input" type="checkbox" id="${safeGame}"><label class="form-check-label" for="${safeGame}"></label><label for="${safeGame}">${game}<br></label>
                    </div>`;
        safeGameNames.push(safeGame);
    }

                //@foreach (var item in AData.EnabledPlatformSorted())
                //{
                //
                //}
    html += "</div></div></div>";
    $("#modal_contents").append(html);

    for (const x in safeGameNames) {
        const game = safeGameNames[x];
        $(`#${game}`).change(async function () {
            await toggleGameStats(game, this.checked);
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
    const game = $(`label[for='${safeGame}']:last`).text();
    const accountId = selected.attr("id");
    console.log(game, isChecked);

    if (!isChecked) {
        // Unchecked: Remove entry and continue.
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `DisableGame`, game, accountId);
        ShowGameStatsSetup();
        return;
    }

    // Checked: Get required variables and present to user.
    showGameStatsVars(game);
}

// Open the variable setting menu for game stats for account.
// This is the Manage button, when variables are already set.
async function showGameStatsVars(game) {
    if (!getSelected()) return;
    const accountId = selected.attr("id");

    // Checked: Get required variables and present to user.
    const required = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GetRequiredVars`, game);
    const existing = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GetExistingVars`, game, accountId);
    const hidden = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GetHiddenMetrics`, game, accountId);
    const globallyHidden = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GetGloballyHiddenMetrics`, game);
    showGameVarCollectionModel(game, required, existing, hidden, globallyHidden);
}

async function showGameVarCollectionModel(game, requiredVars, existingVars = {}, hidden = {}, globallyHidden = []) {
    if (!getSelected()) return;
    const accountId = selected.attr("id");
    const currentPage = await getCurrentPageFullname();

    const modalTitle = await GetLangSub("Modal_Title_GameVars", { game: game }),
        modalHeading = await GetLangSub("Modal_GameVars_Header", { game: game, username: getDisplayName(), platform: currentPage }),
        submit = await GetLang("Submit"),
        disabledGlobally = await GetLang("Tooltip_DisabledGlobally"),
        metricsToShow = await GetLang("Stats_MetricsToShow");


    $("#modalTitle").text(modalTitle);
    $("#modal_contents").empty();
    let html = "";
    html += `<div class="gameStatsWindow">
                <p>${modalHeading}</p>
                <div class="modalScrollSection centeredContainer">
                    <div class="centeredSection">`;

    let checkboxesMarkup = "";

    for (let [key, value] of Object.entries(requiredVars)) {
        console.log(key, value);
        let placeholder = "";
        if (value.includes("[") && value.includes("]")) {
            const parts = value.split("[");
            value = parts[0].trim();
            placeholder = parts[1].trim().replace("]", "");
        }

        let existingValue = key in existingVars ? existingVars[key] : "";
        if (value === "%ACCOUNTID%") {
            value = await GetLang("Stats_AccountId");
            if (existingValue === "")
                existingValue = accountId;
        }

        html +=
            `<div class="rowSetting"><span>${value}</span><input type="text" id="acc${key}" spellcheck="false" placeholder="${placeholder}" value="${existingValue}"></div>`;
    }

    for (let [key, value] of Object.entries(hidden)) {
        const metricHidden = value["item1"], checkboxText = value["item2"];
        let disabled = false;
        if (globallyHidden.includes(key)) disabled = true;
        checkboxesMarkup += `<div class="form-check mb-2" ${disabled ? "data-toggle=\"tooltip\" title=\"" + disabledGlobally + "\"" : ""}><input class="form-check-input" type="checkbox" id="${key}" ${(!metricHidden ? "checked" : "")} ${disabled ? "disabled" : ""}><label class="form-check-label" for="${key}"></label><label for="${key}">${checkboxText}<br></label></div>`;
    }

    html += `       </div>
                </div>
                <div>
                    <p>${metricsToShow}</p>
                    ${checkboxesMarkup}
                </div>
                <div class="settingsCol inputAndButton">
                    <button class="modalOK" type="button" id="set_password" onclick="Modal_FinaliseGameVars('${game}', '${accountId}')"><span>${submit}</span></button>
                </div>
            </div>`;
    $("#modal_contents").append(html);
    $("#modalBtnBack").show();
    $("#modalBtnBack").on("click",
        () => {
            $("#modalBtnBack").hide();
            ShowGameStatsSetup();
        });

    $(".modalBG").fadeIn(() => {
        try {
            initTooltips();
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
    const currentPage = await getCurrentPageFullname();

    // Get list of variable keys
    const requiredVars = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GetRequiredVars`, game);
    const possibleHidden = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GetHiddenMetrics`, game, accountId);
    const globallyHidden = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GetGloballyHiddenMetrics`, game);
    console.log(requiredVars, typeof (requiredVars));

    // Get value for each key and create dictionary
    const returnDict = {};
    for (const [key, value] of Object.entries(requiredVars)) {
        console.log(key, value, $(`#acc${key}`).val());
        returnDict[key] = $(`#acc${key}`).val();
    }

    // Get list of hidden metrics
    const hidden = [];
    for (const [key, _] of Object.entries(possibleHidden)) {
        const checkbox = $(`#${key}`);
        if (checkbox.is(":not(:checked)")) {
            hidden.push(key);
        }
    }

    // Add user statistics for game, with collected variables
    $(".modalBG").fadeOut();
    const success = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `SetGameVars`, currentPage, game, accountId, returnDict, hidden);
    if (success) location.reload();
}

async function refreshAccount(game, accountId) {
    const currentPage = await getCurrentPageFullname();
    $(".modalBG").fadeOut();
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `RefreshAccount`, accountId, game, currentPage);
    location.reload();
}



Modal_FinalizeImage = async(dest) => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `ImportNewImage`, JSON.stringify({ dest: dest, path: $("#FolderLocation").val() }));

// Open Steam\userdata\<steamID> folder
async function openUserdata(e) {
    if (e !== undefined && e !== null) e.preventDefault();
    if (!getSelected()) return;

    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `SteamOpenUserdata`, selected.attr("id"));
}

// Create shortcut for selected icon
async function createShortcut(args = '') {
    const selected = $(".acc:checked");
    if (selected === "" || selected[0] === null || typeof selected[0] === "undefined") {
        return;
    }
    const accId = selected.attr("id");

    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server",
        "CreateShortcut",
        getCurrentPage(),
        accId,
        getDisplayName(),
        args);
}

// NEW LOGIN
newLogin = async() => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `NewLogin_${getCurrentPage()}`);
hidePlatform = async() => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "HidePlatform", selectedElem);
createPlatformShortcut = async() => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCreatePlatformShortcut", selectedElem);

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
    const r = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiExportAccountList", selectedElem);
    const filename = r.split("/");
    saveFile(filename[filename.length - 1], r);
    exportingAccounts = false;
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

// Link handling
OpenLinkInBrowser = async(link) => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "OpenLinkInBrowser", link);

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
            modalInfoViewPatreon = await GetLang("Modal_Info_ViewPatreon"),
            modalInfoViewKofi = await GetLang("Modal_Info_ViewKofi"),
            modalInfoViewGitHub = await GetLang("Modal_Info_ViewGitHub"),
            modalTitleInfo = await GetLang("Modal_Title_Info");

        $("#modalTitle").text(modalTitleInfo);
        $("#modal_contents").empty();

        const currentVersion = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiGetVersion");
        $("#modal_contents").append(`<div class="infoWindow">
                <div class="imgDiv"><img width="100" margin="5" src="img/TcNo500.png" draggable="false" onclick="OpenLinkInBrowser('https://tcno.co');"></div>
                <div class="rightContent">
                    <h2>TcNo Account Switcher</h2>
                    <p>${modalInfoCreator}</p>
                    <div class="linksList">
                        <a onclick="OpenLinkInBrowser('https://patreon.com/TroubleChute');return false;" href=""><svg viewBox="0 0 24 24" draggable="false" alt="GitHub" class="modalIcoPatreon"><use href="img/icons/ico_patreon.svg#icoPatreon"></use></svg>${modalInfoViewPatreon}</a>
                        <a onclick="OpenLinkInBrowser('https://ko-fi.com/tcnoco');return false;" href=""><svg viewBox="0 0 24 24" draggable="false" alt="GitHub" class="modalIcoKofi"><use href="img/icons/ico_kofi.svg#icoKofi"></use></svg>${modalInfoViewKofi}</a>
                        <a onclick="OpenLinkInBrowser('https://github.com/TcNobo/TcNo-Acc-Switcher');return false;" href=""><svg viewBox="0 0 24 24" draggable="false" alt="GitHub" class="modalIcoGitHub"><use href="img/icons/ico_github.svg#icoGitHub"></use></svg>${modalInfoViewGitHub}</a>
                        <a onclick="OpenLinkInBrowser('https://s.tcno.co/AccSwitcherDiscord');return false;" href=""><svg viewBox="0 0 24 24" draggable="false" alt="Discord" class="modalIcoDiscord"><use href="img/icons/ico_discord.svg#icoDiscord"></use></svg>${modalInfoBugReport}</a>
                        <a onclick="OpenLinkInBrowser('https://tcno.co');return false;" href=""><svg viewBox="0 0 24 24" draggable="false" alt="Website" class="modalIcoNetworking"><use href="img/icons/ico_networking.svg#icoNetworking"></use></svg>${modalInfoVisitSite}</a>
                        <a onclick="OpenLinkInBrowser('https://github.com/TcNobo/TcNo-Acc-Switcher/blob/master/DISCLAIMER.md');return false;" href=""><svg viewBox="0 0 2084 2084" draggable="false" alt="GitHub" class="modalIcoDoc"><use href="img/icons/ico_doc.svg#icoDoc"></use></svg>${modalInfoDisclaimer}</a>
                    </div>
                </div>
                </div><div class="versionIdentifier"><span>${modalInfoVersion}: ${currentVersion}</span></div>`);
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
            extraButtons = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "PlatformUserModalExtraButtons");
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
        platform = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCurrentBasicPlatform", platform);
        const currentBasicPlatformExe =
            await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCurrentBasicPlatformExe", platform);
        platformExe = platform === currentBasicPlatformExe ? platformExe : currentBasicPlatformExe;

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
        } else if (action.startsWith("AcceptForget")) {
            // Split action after AcceptForget and before Acc:
            const platform = action.split("AcceptForget")[1].split("Acc")[0];
            message = await GetLangSub("Prompt_ForgetAccount", { platform });
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

        $("#modalTitle").text(await GetLangSub("Modal_Title_AccountNotes", { accountName: getDisplayName() }));
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

        input = document.getElementById("accNotes");
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
        const extraButtons = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "PlatformUserModalExtraButtons");

        Modal_RequestedLocated(false);
        // Sub in info if this is a basic page
        var redirectLink = platform;
        platform = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCurrentBasicPlatform", platform);

        const modalTitleAddNew = await GetLangSub("Modal_Title_AddNew", { platform: platform }),
            modalAddNew = await GetLangSub("Modal_AddNew", { platform: platform }),
            modalAddCurrentAccount = await GetLangSub("Modal_AddCurrentAccount", { platform: platform });

        $("#modalTitle").text(modalTitleAddNew);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div>
		        <span class="modal-text">${modalAddNew}</span>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" minlength="1" id="CurrentAccountName" autocomplete="off" style="width: 100%;padding: 8px;" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('set_account_name').click();" onkeypress='return /[^<>:\\.\\"\\/\\\\|?*]/i.test(event.key)'>
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
        let modalTitle = await GetLang("Modal_Title_Stats");

        const modalText = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GetStatsString");

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
            if (input.nodeName !== "TEXTAREA") input.select();
        }
        catch (err) {

        }
    });
}

async function getLogicalDrives() {
    var folderContent = "";
    const logicalDrives = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GetLogicalDrives");
    folderContent = "<div>";
    logicalDrives.Folders.forEach((f) => { folderContent += "<span class=\"folder\" path=\"" + f + "\">" + f + "</span>"; });
    folderContent += "</div>";

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

        const fileSystemResult = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", getFunc, result);
        folderContent = "<div path=\"" + currentSpanPath + "\"><span class=\"folder c head selected-path\" path=\"" + currentSpanPath + "\">" + (currentSpanPath.at(-1) !== "\\" ? currentSpanPath.split("\\").at(-1) : currentSpanPath) + "</span>";
        fileSystemResult.Folders.forEach((f) => {
            folderContent += "<span class=\"folder\" path=\"" + f + "\">" + (f.at(-1) !== "\\" ? f.split("\\").at(-1) : f) + "</span>";
        });
        fileSystemResult.Files.forEach((f) => {
            folderContent += "<span " + (f.includes(pathPickerRequestedFile) ? "class=\"suggested\" " : "") + "path=\"" + f + "\">" + (f.at(-1) !== "\\" ? f.split("\\").at(-1) : f) + "</span>";
        });
        folderContent += "</div>";

        //console.log(folderContent);
        $(e.target).replaceWith(folderContent);

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

    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiUpdatePath", platformSettingsPath, $("#FolderLocation").val());
    $(".modalBG").fadeOut();

    location.reload();
}

async function Modal_Confirm(action, value) {
    if (window.location.href.includes("PreviewCss")) {
        // Do nothing for CSS preview page.
        return;
    }

    const success = await GetLang("Success");
    const confirmAction = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiConfirmAction", action, value);
    if (confirmAction === "refresh") location.reload();
    else if (confirmAction === "success")
        window.notification.new({
            type: "success",
            title: "",
            message: success,
            renderTo: "toastarea",
            duration: 3000
        });

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
        name = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiGetCleanFilePath", raw);
    }

    // If length 0, return with error:
    if (name.trim().length === 0) {
        const toastNameEmpty = await GetLang("Toast_NameEmpty"),
            error = await GetLang("Error");

        window.notification.new({
            type: "error",
            title: error,
            message: toastNameEmpty,
            renderTo: "toastarea",
            duration: 5000
        });
        return;
    }

    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", platform + "AddCurrent", name);
    $(".modalBG").fadeOut();
    $("#acc_list").click();
}

async function Modal_FinaliseBackground() {
    const pathOrUrl = $("#FolderLocation").val();
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SetBackground", pathOrUrl);
    $(".modalBG").fadeOut();
}

async function Modal_FinaliseSwitcherPassword() {
    const switcherPassword = $("#SwitcherPassword").val();
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SetSwitcherPassword", switcherPassword);
    $(".modalBG").fadeOut();
}

async function Modal_FinaliseUserDataFolder() {
    if (window.location.href.includes("PreviewCss")) {
        // Do nothing for CSS preview page.
        return;
    }

    const pathOrUrl = $("#FolderLocation").val();
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SetUserData", pathOrUrl);
    $(".modalBG").fadeOut();
}

async function Modal_FinaliseAccNameChange() {
    if (window.location.href.includes("PreviewCss")) {
        // Do nothing for CSS preview page.
        return;
    }

    const raw = $("#NewAccountName").val();
	const name = (raw.indexOf("TCNO:") === -1 ? raw.replace(/[<>: \.\"\/\\|?*]/g, "-") : raw); // Clean string if not a command string.
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "ChangeUsername", $(".acc:checked").attr("id"), name);
}

async function Modal_SaveNotes(accId) {
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `Set${getCurrentPage()}Notes`, accId, $('#accNotes').val());
    location.reload(true);
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

async function usernameModalCopyText() {
    const toastTitle = await GetLang("Toast_Copied");
    const platform = getCurrentPage();

    const toastHintText = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "PlatformHintText", platform);
    const code = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "PlatformUserModalCopyText");

    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "CopyToClipboard", code);
    window.notification.new({
	    type: "success",
        title: toastTitle,
        message: toastHintText,
	    renderTo: "toastarea",
	    duration: 5000
    });
}


// Basic account switcher: Shortcut dropdown
var sDropdownInitialized = false;
function sDropdownReposition() {
    const dropDownContainer = $("#shortcutDropdown");
    const btn = $("#shortcutDropdownBtn");
    const dropDownItemsContainer = $(".shortcutDropdownItems")[0];
    const btnPos = btn[0].getBoundingClientRect();
    dropDownContainer.css({ top: btnPos.top - dropDownContainer.height() - btn.height() - 16, left: btnPos.left + 16 - (dropDownContainer.width() / 2) });

    // If overflowing - Widen by scrollbar width to prevent weird overflow gap on side
    if (checkOverflow(dropDownItemsContainer) && dropDownContainer[0].style.minWidth === '') {
        const scrollbarWidth = (dropDownItemsContainer.offsetWidth - dropDownItemsContainer.clientWidth);
        const hasContextMenu = $(".HasContextMenu");
        if (hasContextMenu.length > 0) {
            const computedStyle = window.getComputedStyle($(".HasContextMenu")[0]);
            const computedStyleContainer = window.getComputedStyle($("#shortcutDropdown")[0]);
            const marginX = parseInt(computedStyle.marginLeft) + parseInt(computedStyle.marginRight);
            const marginY = parseInt(computedStyle.marginBottom) + parseInt(computedStyle.marginTop);
            const paddingX = parseInt(computedStyleContainer.paddingLeft) + parseInt(computedStyleContainer.paddingRight);
            const paddingY = parseInt(computedStyleContainer.paddingTop) + parseInt(computedStyleContainer.paddingBottom);
            dropDownContainer.css({
                minWidth: $(dropDownItemsContainer).width() + scrollbarWidth + marginX + paddingX
            });
            $(dropDownItemsContainer).css({
                maxHeight: dropDownContainer.height() - paddingY + marginY
            });
        }
    }
}

function sDropdownInit() {
    if (sDropdownInitialized) return;
    sDropdownInitialized = true;
    // Create sortable list
    sortable(".shortcuts, .shortcutDropdownItems", {
        connectWith: "shortcutJoined",
        forcePlaceholderSize: true,
        placeholderClass: "shortcutPlaceholder",
        items: ":not(#btnOpenShortcutFolder)"
    });

    $(".shortcuts, .shortcutDropdownItems").toArray().forEach(el => {
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
    const curOverflow = el.style.overflow;

    if (!curOverflow || curOverflow === "visible")
        el.style.overflow = "hidden";

    const isOverflowing = el.clientWidth < el.scrollWidth
        || el.clientHeight < el.scrollHeight;

    el.style.overflow = curOverflow;

    return isOverflowing;
}

function shortcutDropdownBtnClick() {
    if (!$("#shortcutDropdown").is(":visible")) {
        sDropdownInit();
        $("#shortcutDropdown").show();
        sDropdownReposition();
        $("#shortcutDropdownBtn").addClass("flip");
        // If has no children in main list, add the expandShortcuts CSS to show users they can drag.
        if ($(".shortcuts button").length === 0) {
            $(".shortcuts").addClass("expandShortcuts");
        }
    } else {
        $("#shortcutDropdown").hide();
        $("#shortcutDropdownBtn").removeClass("flip");
        $(".shortcuts").removeClass("expandShortcuts");
    }
}

async function serializeShortcuts() {
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
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SaveShortcutOrderSteam", output);
    else
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SaveShortcutOrder", output);
}

// Context menu buttons
async function shortcut(action) {
    const reqId = $(selectedElem).prop("id");
    console.log(reqId);
    if (getCurrentPage() === "Steam")
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "HandleShortcutActionSteam", reqId, action);
    else
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "HandleShortcutAction", reqId, action);
    if (action === "hide") $(selectedElem).remove();
}

updateBarClick = async () => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "UpdateNow");

async function initSavingHotKey() {
    hotkeys("ctrl+s", async function (event) {
        event.preventDefault();
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCtrlS", getCurrentPage());
    });
}

getDisplayName = () => $(selectedElem).siblings("label").find(".displayName").text();

async function initCopyHotKey() {
    const toastCopied = await GetLang("Toast_Copied");
    hotkeys("ctrl+c,ctrl+shift+c,alt+c", async function (event, handler) {
        // Doesn't prevent default!
        switch (handler.key) {
        case "ctrl+shift+c":
        case "alt+c":
            await copyToClipboard($(selectedElem).prop("id"));
            break;
        case "ctrl+c":
                await copyToClipboard(getDisplayName());
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

restartAsAdmin = async(args = "") => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiRestartAsAdmin", args);