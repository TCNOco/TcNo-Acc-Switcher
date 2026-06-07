https://tcno.co/Projects/AccSwitcher/TcNo.Account.Switcher.Core.x64.zip|https://tcno.co/Projects/AccSwitcher/TcNo.Account.Switcher.Core.x32.zip
<?php
/*
    GOAL:
    Collect the number of program "uses" that are updating from each country, eventually to showcase the program's popularity, for interest's sake.
*/
// Taken from: https://stackoverflow.com/questions/12553160/getting-visitors-country-from-their-ip/13600004#13600004
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
$filename = __DIR__ ."/NetCore_updatedata/".date("Y-m-d").".json";
$jsToday = json_decode(@file_get_contents($filename), true); // "@" ignores non-exist error

$currentCount = 0;

$jsToday["total"] += 1; // Total users +1
///$jsToday["Africa"]["South Africa"]["Gauteng"]
$jsToday[$details["continent_code"]][$details["country"]][$details["state"]] += 1; // Get and incriment value, set value in array if not already set

///print_r($jsToday);
file_put_contents($filename, json_encode($jsToday));
?>