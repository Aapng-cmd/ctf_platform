<?php
require_once "config.php";
$conn = start_conn();

$user_info = get_user_info($conn);
if ($user_info['group_type'] !== 2) {
    echo json_encode(['success' => false, 'message' => 'Not an admin']);
    header("Location: home.php");
    exit;
}


$data = json_decode(file_get_contents("php://input"), true);

if (isset($data['id'], $data['description'], $data['readme'], $data['flag'], $data['hints'])) {
    // Update task details
    $stmt = $conn->prepare("UPDATE tasks SET description = ?, readme = ?, flag = ? WHERE id = ?");
    $stmt->bind_param("sssi", $data['description'], $data['readme'], $data['flag'], $data['id']);
    
    if ($stmt->execute()) {
        // Update hints
        foreach ($data['hints'] as $hint) {
            if (isset($hint['id'])) {
                // Update existing hint
                $stmt = $conn->prepare("UPDATE hints SET hint = ?, cost = ? WHERE id = ?");
                $stmt->bind_param("sii", $hint['text'], $hint['cost'], $hint['id']);
            } else {
                // Insert new hint
                $stmt = $conn->prepare("INSERT INTO hints (task_id, description, cost) VALUES (?, ?, ?)");
                $stmt->bind_param("isi", $data['id'], $hint['text'], $hint['cost']);
            }
            $stmt->execute();
            $stmt->close();
        }
        echo json_encode(['success' => true]);
    } else {
        echo json_encode(['success' => false]);
    }
    
    $stmt->close();
}

$conn->close();
?>
