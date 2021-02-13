const remote = require('electron').remote;

//window.onbeforeunload = (event) => {
//    win.removeAllListeners();
//}

function handleWindowControls() {
    console.log("HandleWindowControls() was called!");
    document.getElementById('btnMin').addEventListener("click", event => {
        console.log("Minimizing window!");
        remote.getCurrentWindow().minimize();
    });

    document.getElementById('btnMax').addEventListener("click", event => {
        console.log("Maximizing window!");
        remote.getCurrentWindow().maximize();
    });

    document.getElementById('btnRestore').addEventListener("click", event => {
        console.log("Restoring window!");
        remote.getCurrentWindow().unmaximize();
    });

    document.getElementById('btnClose').addEventListener("click", event => {
        console.log("Closing window!");
        remote.getCurrentWindow().close();
    });

    toggleMaxRestoreButtons();
    remote.getCurrentWindow().on('maximize', toggleMaxRestoreButtons);
    remote.getCurrentWindow().on('unmaximize', toggleMaxRestoreButtons);

    function toggleMaxRestoreButtons() {
        if (remote.getCurrentWindow().isMaximized()) {
            document.body.classList.add('maximized');
        } else {
            document.body.classList.remove('maximized');
        }
    }
}