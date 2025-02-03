<?php
require_once "config.php"; // Include your database configuration


is_logged();

if (isset($_GET['hint_id']))
{
    $hint_id = intval($_GET['hint_id']);
    $conn = start_conn();
    $stmt = $conn->prepare("SELECT description FROM hints WHERE id = ?");
    $stmt->bind_param("i", $hint_id);
    $stmt->execute();
    $stmt->store_result();
    $stmt->bind_result($description);
    if($stmt->fetch())
        echo json_encode(['success' => true, 'description' => $description]);
    else
        echo json_encode(['success' => false, 'description' => 'Проблемка, мда']);
    $stmt->close();
    
    $stmt = $conn->prepare("
		INSERT INTO user_task_costs (user_id, hint_id) VALUES (?, ?)
	");

	$stmt->bind_param("ii", $_SESSION['id'], $hint_id);
	$stmt->execute();
	$stmt->close();
	$conn->close();
}


?>
