
// Allow multiple users to be selected.
// Much easier than people having to hold Ctrl, then click on items to select multiple.
$("select[name='IgnoredBattleNetAccounts']").mousedown(function (e) {
    e.preventDefault();

    const s = this;
    const scroll = s.scrollTop;

    e.target.selected = !e.target.selected;

    setTimeout(function () { s.scrollTop = scroll; }, 0);

    $(s).focus();
}).mousemove(function (e) { e.preventDefault() });


// Set list of users
function setIgnored(listIgnored) {
    const listAccounts = document.getElementById("IgnoredAccounts");

    var ignoredUsers = JSON.parse(listIgnored);
    Object.keys(ignoredUsers).forEach(function (key) {
        $(listAccounts).append(`<option value="${key}">${ignoredUsers[key]} [${key}]</option>`);
    })
}
// Load list of users
export function jsLoadIgnored() {
    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiFileReadAllText", "LoginCache\\BattleNet\\IgnoredAccounts.json").then(r => {
        console.log("GOT IGNORED USERS");
        console.log(r);
        setIgnored(r);
    });
    console.log("Getting forgotten users from file");
}