<?php
require_once "config.php";
$conn = start_conn();

is_logged();

$user_info = get_user_info($conn);


// Check if the request method is POST
if ($_SERVER['REQUEST_METHOD'] === 'GET' && isset($_GET['task_id']) && isset($_GET['rating'])) {
    // Get the task ID and rating from the POST request
    $task_id = intval($_GET['task_id']);
    $rating = intval($_GET['rating']);

    $stmt = $conn->prepare("SELECT task_id FROM solved_tasks WHERE task_id = ? AND user_id = ?");
    $stmt->bind_param("ii", $task_id, $user_info['id']);
    $stmt->execute();
    $result = $stmt->get_result();

    if ($result->num_rows === 0) {
        echo "Ошибка: Задача не найдена.";
        $stmt->close();
        exit;
    }
    
    $stmt = $conn->prepare("SELECT task_id FROM user_rated_task WHERE task_id = ? AND user_id = ?");
    $stmt->bind_param("ii", $task_id, $user_info['id']);
    $stmt->execute();
    $result = $stmt->get_result();

	if ($result->num_rows !== 0) {
        echo "Ошибка: Уже оценивал задачу.";
        $stmt->close();
        exit;
    }

    // Update the rating in the tasks_ratings table
    $stmt = $conn->prepare("INSERT INTO tasks_ratings (task_id, rating) VALUES (?, ?)
                             ON DUPLICATE KEY UPDATE rating = (rating + ?)");
    $stmt->bind_param("iii", $task_id, $rating, $rating);
    
    $stmt_user = $conn->prepare("INSERT INTO user_rated_task (task_id, user_id) VALUES(?, ?)");
    $stmt_user->bind_param("ii", $task_id, $user_info['id']);
    
    if ($stmt_user->execute() && $stmt->execute()) {
        echo "success";
    } else {
        echo "Ошибка: Не удалось обновить оценку.";
    }
	
	$stmt_user->close();
    $stmt->close();
} else {
    echo "Ошибка: Неверный метод запроса.";
}

$conn->close();

?>
