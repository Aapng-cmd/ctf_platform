<?php
require_once "config.php";

$conn = start_conn();
session_start();

if (isset($_GET['id'])) {
    $conn = start_conn();
    $taskId = intval($_GET['id']);
    
    // Fetch task details including first blood user
    $query = "
        SELECT 
            tasks.name, 
            tasks.cost, 
            tasks.description,
            tasks.hosting,
            tasks.files,
            COUNT(solved_tasks.user_id) AS solutions_count, 
            users.username AS first_blood_user
        FROM tasks
        LEFT JOIN solved_tasks ON tasks.id = solved_tasks.task_id
        LEFT JOIN users ON solved_tasks.first_blood_id = users.id
        WHERE tasks.id = ?
        GROUP BY tasks.id, tasks.name, tasks.cost, tasks.description, tasks.hosting, users.username
    ";
    
    $stmt = $conn->prepare($query);
    $stmt->bind_param('i', $taskId);
    $stmt->execute();
    $result = $stmt->get_result();

    if ($task = $result->fetch_assoc()) {
        echo json_encode(['success' => true, 'task' => $task]);
    } else {
        echo json_encode(['success' => false, 'message' => 'Нет такой задачи']);
    }

    $stmt->close();
} else {
    echo json_encode(['success' => false, 'message' => 'No task ID provided']);
}

$conn->close();
?>
