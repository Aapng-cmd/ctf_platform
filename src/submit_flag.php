<?php
require_once "config.php";
$conn = start_conn();
session_start();

if (!is_logged())
{
    header("Location: login.php");
    exit;
}

function validate_flag($conn, $flag, $task)
{
	$stmt = $conn->prepare('SELECT user_id FROM solved_tasks WHERE user_id = ? AND task_id = (SELECT id FROM tasks WHERE name = ?)');
    $stmt->bind_param('is', $_SESSION['id'], $task);
    $stmt->execute();
    $stmt->store_result();
    $stmt->bind_result($id);
    $stmt->fetch();
    $stmt->close();
    if (!($id === null))
    {
    	return 'Задача уже решена';
    }
    
    // Prepare the first statement to get the valid flag
    $stmt = $conn->prepare("SELECT flag FROM tasks WHERE id = (SELECT id FROM tasks WHERE name = ?)");
    $stmt->bind_param("s", $task);
    $stmt->execute();
    $stmt->bind_result($valid_flag);
    
    // Fetch the result
    if ($stmt->fetch()) {
        // Check if the provided flag matches the valid flag
        if ($flag === $valid_flag) {
            // Close the first statement before preparing the second one
            $stmt->close();

            // Prepare the second statement to insert into solved_tasks
            $stmt = $conn->prepare("INSERT INTO solved_tasks (user_id, task_id) VALUES (?, (SELECT id FROM tasks WHERE name = ?))");
            $stmt->bind_param("is", $_SESSION['id'], $task);
            $stmt->execute();
            $stmt->close();
            return "success";
        }
    }
    
    // Close the first statement if it was not successful
    $stmt->close();
    return "fail";
}

if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_POST['flag']) && isset($_POST['task']))
{
    echo validate_flag($conn, $_POST['flag'], $_POST['task']);
}

$conn->close();
?>
