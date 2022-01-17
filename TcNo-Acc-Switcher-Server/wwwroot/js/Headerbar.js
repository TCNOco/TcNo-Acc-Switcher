if (chrome === undefined) {
	window.notification.new({
		type: "error",
        title: "",
		message: "A critical component could not be loaded (chrome). Please restart the application!",
		renderTo: "toastarea",
		duration: 10000
	});
	chrome = null;
}


const d = new Date();
var monthDay = "";

function getDate() {
    if (monthDay === "") monthDay = d.getMonth().toString() + d.getDate().toString();
    return monthDay;
}

const SysCommandSize = { // Reverses for april fools
    ScSizeHtLeft: (getDate() !== "31" ? 0xA : 0xB), // 1 + 9
    ScSizeHtRight: (getDate() !== "31" ? 0xB : 0xA),
    ScSizeHtTop: (getDate() !== "31" ? 0xC : 0xF),
    ScSizeHtTopLeft: (getDate() !== "31" ? 0xD : 0x11),
    ScSizeHtTopRight: (getDate() !== "31" ? 0xE : 0x10),
    ScSizeHtBottom: (getDate() !== "31" ? 0xF : 0xC),
    ScSizeHtBottomLeft: (getDate() !== "31" ? 0x10 : 0xE),
    ScSizeHtBottomRight: (getDate() !== "31" ? 0x11 : 0xD),

    ScMinimise: 0xF020,
    ScMaximise: 0xF030,
    ScRestore: 0xF120
};
const WindowNotifications = {
    WmClose: 0x0010
};

var possibleAnimations = [
    "Y",
    "X",
    "Z"
];

function btnBack_Click() {
    if (window.location.pathname === "/") {
        $("#btnBack i").css({ "transform": "rotate" + possibleAnimations[Math.floor(Math.random() * possibleAnimations.length)] + "(360deg)", "transition": "transform 500ms ease-in-out" });
        setTimeout(() => $("#btnBack i").css({ "transform": "", "transition": "transform 0ms ease-in-out" }), 500);
    }
    else {
	    const tempUri = document.location.href.split("?")[0];
	    document.location.href = tempUri + (tempUri.endsWith("/") ? "../" : "/../");
    }
}

function handleWindowControls() {
    document.getElementById("btnBack").addEventListener("click", () => {
        btnBack_Click();
    });

    if (navigator.appVersion.indexOf("TcNo") === -1) return;

    if (navigator.appVersion.indexOf("TcNo-CEF") !== -1) {
        document.getElementById("btnMin").addEventListener("click", () => {
            CefSharp.PostMessage({ "action": "WindowAction", "value": SysCommandSize.ScMinimise });
        });

        document.getElementById("btnMax").addEventListener("click", () => {
            CefSharp.PostMessage({ "action": "WindowAction", "value": SysCommandSize.ScMaximise });
        });

        document.getElementById("btnRestore").addEventListener("click", () => {
            CefSharp.PostMessage({ "action": "WindowAction", "value": SysCommandSize.ScRestore });
        });

        document.getElementById("btnClose").addEventListener("click", () => {
            DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GetTrayMinimizeNotExit").then((r) => {
                if (r && !event.ctrlKey) { // If enabled, and NOT control held
                    CefSharp.PostMessage({ "action": "HideWindow" });
                } else {
                    CefSharp.PostMessage({ "action": "WindowAction", "value": WindowNotifications.WmClose });
                }
            });
        });

    }
    else // The normal WebView browser
    {
        document.getElementById("btnMin").addEventListener("click", () => {
            chrome.webview.hostObjects.sync.eventForwarder.WindowAction(SysCommandSize.ScMinimise);
        });

        document.getElementById("btnMax").addEventListener("click", () => {
            chrome.webview.hostObjects.sync.eventForwarder.WindowAction(SysCommandSize.ScMaximise);
        });

        document.getElementById("btnRestore").addEventListener("click", () => {
            chrome.webview.hostObjects.sync.eventForwarder.WindowAction(SysCommandSize.ScRestore);
        });

        document.getElementById("btnClose").addEventListener("click", () => {
            DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GetTrayMinimizeNotExit").then((r) => {
                if (r && !event.ctrlKey) { // If enabled, and NOT control held
                    chrome.webview.hostObjects.sync.eventForwarder.HideWindow();
                } else {
                    chrome.webview.hostObjects.sync.eventForwarder.WindowAction(WindowNotifications.WmClose);
                }
            });
        });
    }

    // For draggable regions:
    // https://github.com/MicrosoftEdge/WebView2Feedback/issues/200
    document.body.addEventListener("mousedown", (evt) => {
        // ES is actually 11, set in project file. This error can be ignored (if you see one about ES5)
        const {
            target
        } = evt;
        const appRegion = getComputedStyle(target)["-webkit-app-region"];
        if (evt.button === 0 && appRegion === "drag") {
            if (target.classList.length !== 0) {
                const c = target.classList[0];
                if (c === "headerbar" && navigator.appVersion.indexOf("TcNo-CEF") !== -1) {
                    // User is dragging the title bar, and is on the CEF browser.
                    CefSharp.PostMessage({ "action": "MouseDownDrag" });
                    evt.preventDefault();
                    evt.stopPropagation();
                    return;
                }
                const value = (c === "resizeTopLeft" ? SysCommandSize.ScSizeHtTopLeft : (
                    c === "resizeTop" ? SysCommandSize.ScSizeHtTop : (
                        c === "resizeTopRight" ? SysCommandSize.ScSizeHtTopRight : (
                            c === "resizeRight" ? SysCommandSize.ScSizeHtRight : (
                                c === "resizeBottomRight" ? SysCommandSize.ScSizeHtBottomRight : (
                                    c === "resizeBottom" ? SysCommandSize.ScSizeHtBottom : (
                                        c === "resizeBottomLeft" ? SysCommandSize.ScSizeHtBottomLeft : (
                                            c === "resizeLeft" ? SysCommandSize.ScSizeHtLeft : 0))))))));


                if (navigator.appVersion.indexOf("TcNo-CEF") !== -1) {
                     CefSharp.PostMessage({ "action": "MouseResizeDrag", "value": value });
                }
                else chrome.webview.hostObjects.sync.eventForwarder.MouseResizeDrag(value);
            }

            if (navigator.appVersion.indexOf("TcNo-CEF") === -1) chrome.webview.hostObjects.sync.eventForwarder.MouseDownDrag(); // This breaks resize on CEFSharp for some reason (Drags window instead of resizing - VERY ANNOYING)

            evt.preventDefault();
            evt.stopPropagation();
        }
    });
}