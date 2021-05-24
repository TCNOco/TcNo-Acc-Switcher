//const remote = require('electron').remote;

//window.onbeforeunload = (event) => {
//    win.removeAllListeners();
//}
const d = new Date();
var monthDay = "";
function getDate() { if (monthDay === "") monthDay = d.getMonth().toString() + d.getDate().toString(); return monthDay }

const SysCommandSize = { // Reverses for april fools
    ScSizeHtLeft: (getDate() !== "31" ? 0xA : 0xB), // 1 + 9
    ScSizeHtRight: (getDate() !== "31" ? 0xB : 0xA),
    ScSizeHtTop: (getDate() !== "31" ? 0xC : 0xF),
    ScSizeHtTopLeft: (getDate() !== "31" ? 0xD : 0x11),
    ScSizeHtTopRight: (getDate() !== "31" ? 0xE : 0x10),
    ScSizeHtBottom: (getDate() !== "31" ? 0xF : 0xC),
    ScSizeHtBottomLeft: (getDate() !== "31" ? 0x10 : 0xE) ,
    ScSizeHtBottomRight: (getDate() !== "31" ? 0x11 : 0xD),

    ScMinimise: 0xF020,
    ScMaximise: 0xF030,
    ScRestore: 0xF120
}
const WindowNotifications = {
    WmClose: 0x0010
}

var possibleAnimations = [
    "rotateY",
    "rotateX",
    "rotateZ"
];
var lastDeg = 360;
function RandomAni(e) {
    lastDeg = -lastDeg;
    ani = possibleAnimations[Math.floor(Math.random() * possibleAnimations.length)];

    $({ deg: 0 }).animate({ deg: lastDeg, easing: "swing" }, {
        duration: 500,
        step: function (now) {
            $(e).css({
                transform: ani + "(" + now + "deg)"
            });
        }
    });
}
function handleWindowControls() {
    console.log("HandleWindowControls() was called!");
    document.getElementById("btnMin").addEventListener("click", event => {
        chrome.webview.hostObjects.sync.eventForwarder.WindowAction(SysCommandSize.ScMinimise);
    });

    document.getElementById("btnBack").addEventListener("click", event => {
        if (window.location.pathname === "/") RandomAni("#btnBack .icon");
        else {
            let tempUri = document.location.href.split("?")[0];
            document.location.href = tempUri + (tempUri.endsWith("/") ? "../" : "/../");
        }
    });

    document.getElementById("btnMax").addEventListener("click", event => {
        chrome.webview.hostObjects.sync.eventForwarder.WindowAction(SysCommandSize.ScMaximise);
    });

    document.getElementById("btnRestore").addEventListener("click", event => {
        chrome.webview.hostObjects.sync.eventForwarder.WindowAction(SysCommandSize.ScRestore);
    });

    document.getElementById("btnClose").addEventListener("click", event => {
        var promise = DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GetTrayMinimizeNotExit").then(r => {
            if (r && !event.ctrlKey) { // If enabled, and NOT control held
                chrome.webview.hostObjects.sync.eventForwarder.HideWindow();
            } else {
                chrome.webview.hostObjects.sync.eventForwarder.WindowAction(WindowNotifications.WmClose);
            }
        });
    });

    // For draggable regions:
    // https://github.com/MicrosoftEdge/WebView2Feedback/issues/200
    document.body.addEventListener("mousedown", evt => {
        // ES is actually 11, set in project file. This error can be ignored (if you see one about ES5)
        const { target } = evt;
        const appRegion = getComputedStyle(target)["-webkit-app-region"];
        //const appRegion = getComputedStyle(target);
        //console.log(evt);
        //console.log(appRegion);
        if (evt.button === 0 && appRegion === "drag") {
            if (target.classList.length !== 0) {
                const c = target.classList[0];
                chrome.webview.hostObjects.sync.eventForwarder.MouseResizeDrag(
                    (c === "resizeTopLeft" ? SysCommandSize.ScSizeHtTopLeft : (
                        c === "resizeTop" ? SysCommandSize.ScSizeHtTop : (
                            c === "resizeTopRight" ? SysCommandSize.ScSizeHtTopRight : (
                                c === "resizeRight" ? SysCommandSize.ScSizeHtRight : (
                                    c === "resizeBottomRight" ? SysCommandSize.ScSizeHtBottomRight : (
                                        c === "resizeBottom" ? SysCommandSize.ScSizeHtBottom : (
                                            c === "resizeBottomLeft" ? SysCommandSize.ScSizeHtBottomLeft : ( 
                                                c === "resizeLeft" ? SysCommandSize.ScSizeHtLeft : 0)))))))));
            }

            chrome.webview.hostObjects.sync.eventForwarder.MouseDownDrag();
            
            evt.preventDefault();
            evt.stopPropagation();
        }
    });

    //toggleMaxRestoreButtons();
    // remote.getCurrentWindow().on('maximise', toggleMaxRestoreButtons);
    // remote.getCurrentWindow().on('maximise', toggleMaxRestoreButtons);

    // Not too sure how to do this in the latest version - With WebView2. Will likely have something on the WPF side that execs JS code.
    //function toggleMaxRestoreButtons() {
    //    if (remote.getCurrentWindow().isMaximised()) {
    //        document.body.classList.add('maximised');
    //    } else {
    //        document.body.classList.remove('maximised');
    //    }
    //}
}