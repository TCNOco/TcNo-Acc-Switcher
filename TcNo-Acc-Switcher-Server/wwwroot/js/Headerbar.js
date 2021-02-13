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
        // remote.getCurrentWindow().maximize();
        window.location = "Win_max";
    });

    document.getElementById('btnRestore').addEventListener("click", event => {
        console.log("Restoring window!");
        // remote.getCurrentWindow().unmaximize();
        window.location = "Win_restore";
    });

    document.getElementById('btnClose').addEventListener("click", event => {
        console.log("Closing window!");
        //remote.getCurrentWindow().close();
        window.location = "Win_close";
    });

    toggleMaxRestoreButtons();
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