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

function handleWindowControls() {
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
                const value = (c === "resizeTopLeft" ? SysCommandSize.ScSizeHtTopLeft : (
                    c === "resizeTop" ? SysCommandSize.ScSizeHtTop : (
                        c === "resizeTopRight" ? SysCommandSize.ScSizeHtTopRight : (
                            c === "resizeRight" ? SysCommandSize.ScSizeHtRight : (
                                c === "resizeBottomRight" ? SysCommandSize.ScSizeHtBottomRight : (
                                    c === "resizeBottom" ? SysCommandSize.ScSizeHtBottom : (
                                        c === "resizeBottomLeft" ? SysCommandSize.ScSizeHtBottomLeft : (
                                            c === "resizeLeft" ? SysCommandSize.ScSizeHtLeft : 0))))))));


       
                DotNet.invokeMethodAsync("TcNo-Acc-Switcher", "MouseResizeDrag", value);
            }else{
                DotNet.invokeMethodAsync("TcNo-Acc-Switcher", "MouseDownDrag");
            }

            ///if (navigator.appVersion.indexOf("TcNo-CEF") === -1) chrome.webview.hostObjects.sync.eventForwarder.MouseDownDrag(); // This breaks resize on CEFSharp for some reason (Drags window instead of resizing - VERY ANNOYING)

            evt.preventDefault();
            evt.stopPropagation();
        }
    });
}