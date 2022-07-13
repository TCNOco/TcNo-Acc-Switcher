// Handles complaints about "DotNet/$" might not be defined.
if (window.notification === undefined) {
	window.notification = null;
    // Won't be able to show an error, unfortunately.
}
if (DotNet === undefined) {
	window.notification.new({
		type: "error",
		title: "",
        message: "A critical component could not be loaded (DotNet). Please restart the application!",
		renderTo: "toastarea",
		duration: 10000
    });
	DotNet = null;
}
if ($ === undefined) {
	window.notification.new({
		type: "error",
		title: "",
        message: "A critical component could not be loaded (jQuery). Please restart the application!",
		renderTo: "toastarea",
		duration: 10000
	});
	$ = null;
}

// Set list of users
function setIgnored(listIgnored) {
    const listAccounts = document.getElementById("IgnoredAccounts");

    var ignoredUsers = JSON.parse(listIgnored);
    Object.keys(ignoredUsers).forEach((key) => {
        $(listAccounts).append(`<option value="${key}">${ignoredUsers[key]} [${key}]</option>`);
    });
}