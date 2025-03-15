<?php
require_once "config.php";
is_logged();
$conn = start_conn();

$user_info = get_user_info($conn);
if ($user_info['group_type'] !== 2) {
    echo json_encode(['success' => false, 'message' => 'Not an admin']);
    header("Location: home.php");
    exit;
}

if (isset($_GET['id'])) {
    $taskId = intval($_GET['id']);
    
    // Fetch task details
    $query = "
        SELECT 
            tasks.name, 
            tasks.description,
            tasks.readme,
            tasks.flag,
            tasks.cost,
            COUNT(solved_tasks.user_id) AS solutions_count, 
            users.username AS first_blood_user
        FROM tasks
        LEFT JOIN solved_tasks ON tasks.id = solved_tasks.task_id
        LEFT JOIN users ON solved_tasks.first_blood_id = users.id
        WHERE tasks.id = ?
        GROUP BY tasks.id
    ";
    
    $stmt = $conn->prepare($query);
    $stmt->bind_param('i', $taskId);
    $stmt->execute();
    $result = $stmt->get_result();

    // Fetch hints for the task
    $stmt_hint = $conn->prepare("SELECT id, description, cost FROM hints WHERE task_id = ?");
    $stmt_hint->bind_param("i", $taskId);
    $stmt_hint->execute();
    $stmt_hint->store_result();
    $stmt_hint->bind_result($hint_id, $hint_dscription, $hint_cost);
    $hints = [];
    while ($stmt_hint->fetch()) {
        $hints[] = ['hint_id' => $hint_id, 'hint_dscription' => $hint_dscription, 'hint_cost' => $hint_cost];
    }

    if ($task = $result->fetch_assoc()) {
        $response = [
            'success' => true,
            'task' => [
                'name' => $task['name'],
                'description' => $task['description'],
                'readme' => $task['readme'],
                'flag' => $task['flag'],
                'cost' => $task['cost'],
                'first_blood_user' => $task['first_blood_user'],
                'solutions_count' => $task['solutions_count']
            ],
            'hints' => $hints
        ];
        echo json_encode($response);
    } else {
        echo json_encode(['success' => false, 'message' => 'Task not found']);
    }

    $stmt->close();
    $stmt_hint->close();
} else {
    echo json_encode(['success' => false, 'message' => 'No task ID provided']);
}

$conn->close();
?>
