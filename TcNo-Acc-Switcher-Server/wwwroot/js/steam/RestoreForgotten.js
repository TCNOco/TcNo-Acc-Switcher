// Allow multiple users to be selected.
// Much easier than people having to hold Ctrl, then click on items to select multiple.
$("select[name='ForgottenSteamAccounts']").mousedown((e) => {
    e.preventDefault();

    const s = this;
    const scroll = s.scrollTop;

    e.target.selected = !e.target.selected;

    setTimeout(() => {
        s.scrollTop = scroll;
    }, 0);

    $(s).focus();
}).mousemove((e) => {
    e.preventDefault();
});


// Set list of users
function setForgotten(listForgotten) {
    const listAccounts = document.getElementById("IgnoredAccounts");
    
    JSON.parse(listForgotten).forEach((usr) => {
        $(listAccounts).append(`<option value="${usr.SteamId}">[${usr.SteamUser.AccountName}] ${usr.SteamUser.PersonaName}<restoreSteamId value="${usr.SteamId}"></restoreSteamId></option>`);
    });
}
// Load list of users
export function jsLoadForgotten() {
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiFileReadAllText", "SteamForgotten.json").then((r) => {
        setForgotten(r);
    });

    // Onclick function for the checkbox
    $('#Steam_ForgottenShowId').change(() => {
        const checked = $(this).is(":checked");
        $("restoreSteamId").each((_, e) => {
            $(e).parent().attr("visible-content", checked ? ` | ${$(e).attr("value")}` : "");
        });
    });
}