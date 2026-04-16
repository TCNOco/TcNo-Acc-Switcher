<?php

$inJS = "";
if (file_exists(date("Y-m-d").".txt")){
	$inJS = file_get_contents(date("Y-m-d").".txt");
}else{
	if (file_exists(date('Y-m-d',strtotime("-1 days")).".txt")){
		unlink(date('Y-m-d',strtotime("-1 days")).".txt");
	}
	$ch = curl_init();
	$headers = array(
		'Accept: application/json',
		'Content-Type: application/json',
		'Authorization: Bearer ----------------'
	);
	curl_setopt($ch, CURLOPT_URL, "https://api.crowdin.com/api/v2/projects/463718/members");
	curl_setopt($ch, CURLOPT_HTTPHEADER, $headers);
	curl_setopt($ch, CURLOPT_HEADER, 0);
	$body = '{}';

	curl_setopt($ch, CURLOPT_CUSTOMREQUEST, "GET"); 
	curl_setopt($ch, CURLOPT_POSTFIELDS, $body);
	curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);

	// Timeout in seconds
	curl_setopt($ch, CURLOPT_TIMEOUT, 30);
	$inJS = curl_exec($ch);
	
	file_put_contents(date("Y-m-d").".txt", $inJS);
}

$js = json_decode($inJS, true);

foreach ($js["data"] as $user){
	$text = '<li>'.$user["data"]["username"];
	$in = "";
	if (array_key_exists("permissions", $user["data"])){
		foreach ($user["data"]["permissions"] as $key => $value){
			if ($value == "proofreader"){
				if ($in == ""){
					$in = " - ".$key;
				}else{
					$in = $in.", ".$key;
				}
			}
		}
	}
	print($text.$in."</li>");
}