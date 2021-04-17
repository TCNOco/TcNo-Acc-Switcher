////// Set settings
////function setSettings(settingsJsonString) {
////    var settingsJson = JSON.parse(settingsJsonString);
////    $('input[type="checkbox"]').each(function () { // Iterate over checkboxes
////        $(this).prop("checked", settingsJson[$(this).prop("id")]);
////    });

////    $('input[type="number"]').each(function () { // Iterate over number inputs
////        $(this).val(settingsJson[$(this).prop("id")]);
////    });
////}
////// Load settings
////export function jsLoadSettings() {
////    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiLoadSettings", "SteamSettings").then(r => {
////        console.log("GOT SETTINGS");
////        console.log(r);
////        setSettings(r);
////    });
////    console.log("Getting settings from file");
////}



////// Get settings
////function getSettings() {
////    var settingsJson = {};
////    $('input[type="checkbox"]').each(function () { // Iterate over checkboxes
////        settingsJson[$(this).prop("id")] = $(this).prop("checked");
////    });

////    $('input[type="number"]').each(function () { // Iterate over number inputs
////        settingsJson[$(this).prop("id")] = $(this).val();
////    });
////    return JSON.stringify(settingsJson);
////}
//// Save settings
////export function jsSaveSettings() {
////    //DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiSaveSettings", "SteamSettings", getSettings());
////    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiSaveSettings");
////}

