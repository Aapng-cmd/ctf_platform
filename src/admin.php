<?php
require_once "config.php";
$conn = start_conn();
session_start();

is_logged();

$user_info = get_user_info($conn);
if ($user_info['group_type'] !== 2)
{
	header("Location: home.php");
    exit;
}




$conn->close();
?>
