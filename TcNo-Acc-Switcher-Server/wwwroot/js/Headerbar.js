//const remote = require('electron').remote;

//window.onbeforeunload = (event) => {
//    win.removeAllListeners();
//}

function handleWindowControls() {
    console.log("HandleWindowControls() was called!");
    document.getElementById('btnMin').addEventListener("click", event => {
        console.log("Minimizing window!");
        //// remote.getCurrentWindow().minimize();
        window.location = "Win_min";
    });

    document.getElementById('btnMax').addEventListener("click", event => {
        console.log("Maximizing window!");
        document.body.classList.add('maximized');
        // remote.getCurrentWindow().maximize();
        window.location = "Win_max";
    });

    document.getElementById('btnRestore').addEventListener("click", event => {
        console.log("Restoring window!");
        document.body.classList.remove('maximized');
        // remote.getCurrentWindow().unmaximize();M
        window.location = "Win_restore";
    });

    document.getElementById('btnClose').addEventListener("click", event => {
        console.log("Closing window!");
        //remote.getCurrentWindow().close();
        window.location = "Win_close";
    });

    // For draggable regions:
    // https://github.com/MicrosoftEdge/WebView2Feedback/issues/200
        document.body.addEventListener('mousedown', evt => {
            // ES is actually 11, set in project file. This error can be ignored (if you see one about ES5)
            const { target } = evt;
            const appRegion = getComputedStyle(target)['-webkit-app-region'];
            //const appRegion = getComputedStyle(target);
            console.log(appRegion);
            if (appRegion === 'drag') {
                chrome.webview.hostObjects.sync.eventForwarder.MouseDownDrag();
                evt.preventDefault();
                evt.stopPropagation();
            }
        });

    //toggleMaxRestoreButtons();
    // remote.getCurrentWindow().on('maximize', toggleMaxRestoreButtons);
    // remote.getCurrentWindow().on('unmaximize', toggleMaxRestoreButtons);

    // Not too sure how to do this in the latest version - With WebView2. Will likely have something on the WPF side that execs JS code.
    //function toggleMaxRestoreButtons() {
    //    if (remote.getCurrentWindow().isMaximized()) {
    //        document.body.classList.add('maximized');
    //    } else {
    //        document.body.classList.remove('maximized');
    //    }
    //}
}