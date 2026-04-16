// This will collect and copy:
// Username-0000:{"image":"Link To Profile Picture"}
// Make sure to hit Ctrl+Shift+I in Discord to open the Developer Tools, then go to the Console tab.
// -> Not working? Go to User Settings > Advanced (under APP SETTINGS) > Turn on Developer Mode.
// Once in Console:
// - Look at the top next to the eye icon, and the filter text box.
// - Click the "top ▼" dropdown.
// - Select Electron Isolated Context
// - Paste this code
//
// Once it has run, paste in the TcNo Account Switcher's username text box.

clear();
if (this.localStorage == undefined) {
    console.log("On the Console tab:\n- Click the \"top ▼\" dropdown.\n- Select Electron Isolated Context\n- Paste this code again.");
} else {
    var multiAccountStore = this.localStorage.getItem("MultiAccountStore");
    var accountInfo = JSON.parse(multiAccountStore)["_state"]["users"][0];
    var text = accountInfo["username"] + "-" + accountInfo["discriminator"] + ":" + `{"image":"https://cdn.discordapp.com/avatars/${accountInfo["id"]}/${accountInfo["avatar"]}.webp?size=512"}`;
    console.log("Copied!\nPaste this in the TcNo Account Switcher's username text box.");
    copy(text);
}