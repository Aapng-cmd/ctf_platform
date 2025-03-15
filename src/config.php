<?php
session_start();

//if (session_status() === PHP_SESSION_NONE) {
//    session_start();
//}


function start_conn()
{
    
    $db_host = getenv('MYSQL_HOST') ?: '127.0.0.1';
    $db_username = getenv('MYSQL_USER') ?: 'user_site';
    $db_password = getenv('MYSQL_PASSWORD') ?: 'password';
    $db_name = getenv('MYSQL_DATABASE') ?: 'site';

    $conn = new mysqli($db_host, $db_username, $db_password, $db_name);
    
    if ($conn->connect_error) {
        die("Connection failed: " . $conn->connect_error);
    }

    return $conn;
}

function is_logged()
{
	if (session_status() === PHP_SESSION_NONE) {
		session_start();
	}
	
    if (empty($_SESSION))
    {
    	header("Location: login.php");
    	exit;
    }
}

function get_user_info($conn)
{
    $stmt = $conn->prepare("SELECT * FROM users WHERE id = ?");
    $stmt->bind_param("i", $_SESSION['id']);
    $stmt->execute();
    $result = $stmt->get_result();
    $userInfo = [];
    while ($row = $result->fetch_assoc()) {
        $userInfo[] = $row;
    }
    $stmt->close();
    
    return $userInfo[0] ?? null;
}

//$currentHost = $_SERVER['HTTP_HOST'];

//$desiredDomain = getenv('DOMAIN_NAME') ?: 'localhost';

//if ($currentHost !== $desiredDomain) {
//    $newUrl = 'http://' . $desiredDomain . $_SERVER['REQUEST_URI'];
//    
//    header("Location: $newUrl", true, 301);
//    exit();
//}


?>
