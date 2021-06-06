{
    "updates":{
	"2021-06-05_00": ["Automatic crashlog uploads, (hopefully) No usernames/other info in logs, Adds missing colours into theme & more theming updates", 6003],
	"2021-06-04_01": ["Hotfix for theming", 6003],
	"2021-06-04_00": ["Much easier, more robust theming (Now in YAML). Themes folder now easier to access. Fixed some buttons breaking when going forward and back a lot.", 202625],
	"2021-06-03_00": ["Added theme picker. Fixed some issues with Riot switcher", 97594],
	"2021-06-02_02": ["Hotfix - Fixed Steam, Ubisoft and Origin logins not working since last updates", 57068],
	"2021-06-02_01": ["Fixed issue with spaces in image names for Riot account switcher", 18920],
	"2021-06-02_00": ["Riot Games switcher. Many fixes. Change PFP. Steam: Create shortcuts with specific states (Online, Invis...)", 455123],
	"2021-05-22_00": ["Added Epic Games Launcher account switcher. Some fixes. General UI retouch on landing page.", 3982970],
	"2021-05-20_00": ["Multiple steam cleaning bugfixes. Word wrap in updater. Fixed crash when toggling Start with Windows. Change to updater updating (verifies files before updating)", 102603],
        "2021-05-06_02": ["Fixed updater looking for wrong folder (Hence crash last update)", 6245],
        "2021-05-06_01": ["Fixed tray starting in background, not showing anything (when no users saved)", 15287],
        "2021-05-06_00": ["Multiple Tray fixes. Fixed Steam 'Add New' button.", 3179024],
        "2021-04-25_00": ["BattleNet Overwatch SR feature [Thanks iR3turnZ].\nSorting accounts now save/load properly, reorganise to your own liking.", 123148],
        "2021-04-23_00": ["Minimize to Tray. BattleNet/Origin/Ubisoft: Change account names.\nBattleNet: Forget/Remember accounts + AddNew button.\nOther platforms now show on tray.\nReordering added *doesn't save order just yet.", 108940],
        "2021-04-20_02": ["Platforms work properly as Admin now. Updated Ubisoft account switcher.", 74066],
        "2021-04-20_01": ["Fixed positioning of some items.", 5305],
        "2021-04-20_00": ["Fixed crash on settings icon click. Status now shows as 'Ready' when load complete. Fixed positioning of some items.", 58589],
        "2021-04-19_02": ["Fixed Battle.net icon", 6389],
        "2021-04-19_01": ["Battle.net account switcher (Thanks iR3turnZ). Ubisoft switcher offline mode option.", 108040],
        "2021-04-19_00": ["Added Ubisoft Connect switcher. Fixed crashes on launch (with no error). Other fixes & optimizations.", 159808],
        "2021-04-18_02": ["Fixed origin logo", 6061],
        "2021-04-18_01": ["Added Origin account switcher (Very early beta)", 67141],
        "2021-04-18_00": ["Fixed status text selection.\nFixed creation of icons not working\nFixed updater not working (Full redownload required for this)", 1091051],
        "2021-04-17_00": ["Init-BETA", 0]
    }
}
<?php
/*
    GOAL:
    Collect the number of program update checks, eventually to showcase the program's popularity, for interest's sake.
*/
// ip_info() Taken from: https://stackoverflow.com/questions/12553160/getting-visitors-country-from-their-ip/13600004#13600004
// Collecting user City is a bit more invasive IMO and unnecessary, so that was removed.
// Going to impliment something like https://datamaps.github.io/ when I get to it.

function ip_info($ip = NULL, $purpose = "location", $deep_detect = TRUE) {
    $output = NULL;
    if (filter_var($ip, FILTER_VALIDATE_IP) === FALSE) {
        $ip = $_SERVER["REMOTE_ADDR"];
        if ($deep_detect) {
            if (isset($_SERVER['HTTP_X_FORWARDED_FOR']) && filter_var(@$_SERVER['HTTP_X_FORWARDED_FOR'], FILTER_VALIDATE_IP))
                $ip = $_SERVER['HTTP_X_FORWARDED_FOR'];
            if (isset($_SERVER['HTTP_CLIENT_IP']) && filter_var(@$_SERVER['HTTP_CLIENT_IP'], FILTER_VALIDATE_IP))
                $ip = $_SERVER['HTTP_CLIENT_IP'];
        }
    }
    $purpose    = str_replace(array("name", "\n", "\t", " ", "-", "_"), NULL, strtolower(trim($purpose)));
    $support    = array("country", "countrycode", "state", "region", "location", "address");
    $continents = array(
        "AF" => "Africa",
        "AN" => "Antarctica",
        "AS" => "Asia",
        "EU" => "Europe",
        "OC" => "Australia (Oceania)",
        "NA" => "North America",
        "SA" => "South America"
    );
    if (filter_var($ip, FILTER_VALIDATE_IP) && in_array($purpose, $support)) {
        $ipdat = @json_decode(file_get_contents("http://www.geoplugin.net/json.gp?ip=" . $ip));
        if (@strlen(trim($ipdat->geoplugin_countryCode)) == 2) {
            switch ($purpose) {
                case "location":
                    $output = array(
                        "state"          => @$ipdat->geoplugin_regionName,
                        "country"        => @$ipdat->geoplugin_countryName,
                        "country_code"   => @$ipdat->geoplugin_countryCode,
                        "continent"      => @$continents[strtoupper($ipdat->geoplugin_continentCode)],
                        "continent_code" => @$ipdat->geoplugin_continentCode
                    );
                    break;
            }
        }
    }
    return $output;
}
// Get user info
$details = ip_info("Visitor", "Location");
///echo($details["state"].", ".$details["country"].", ".$details["continent"]."<br>");

// Today's JSON file
$filename = __DIR__ .(!isset($_GET['debug']) ? "/../stats/update/" : "/../statsDEBUG/update/").date("Y-m-d").".json";
if (false === ($data = file_get_contents($filename))) {
    exit(); // Process failed.
} else {
    $jsToday = json_decode($data, true); // "@" ignores non-exist error
}

$currentCount = 0;

$jsToday["total"] += 1; // Total users +1
///$jsToday["Africa"]["South Africa"]["Gauteng"]
$jsToday[$details["continent_code"]][$details["country"]][$details["state"]] += 1; // Get and incriment value, set value in array if not already set
$jsToday["update_from"][$_GET['v']] += 1;

///print_r($jsToday);
file_put_contents($filename, json_encode($jsToday));
?>