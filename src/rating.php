<?php
require_once "config.php";

$conn = start_conn();
session_start();

is_logged();

$user_info = get_user_info($conn);

function get_sorted_users($conn)
{
    $stmt = $conn->prepare("SELECT id, username, score FROM users ORDER BY score");
    $stmt->execute();
    $stmt->store_result();
    $stmt->bind_result($id, $username, $score);
    
    $result = [];
    
    while ($stmt->fetch())
    {
        $result[] = ['id' => $id, 'username' => $username, 'score' => $score];
    }
    
    $stmt->close();
    
    return $result;
}

$users = get_sorted_users($conn);

?>

<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Рейтинг</title>
    <style>
        body {
            font-family: 'Courier New', Courier, monospace;
            background-color: #121212;
            color: #00ffcc;
            padding: 20px;
            margin: 0;
        }
        
        .nav-panel {
            background-color: #1c1c1c;
            padding: 10px;
            text-align: center;
            border-radius: 5px;
            margin-bottom: 20px;
        }

        .nav-panel a {
            color: #00ffcc;
            text-decoration: none;
            margin: 0 15px;
            font-weight: bold;
        }

        .nav-panel a:hover {
            color: #ff007f; /* Change color on hover */
        }

        .rating-container {
            max-width: 800px;
            margin: auto;
            background-color: #1c1c1c;
            padding: 20px;
            border-radius: 10px;
            box-shadow: 0 0 10px rgba(0, 255, 255, 0.3);
        }

        h1 {
            text-align: center;
            color: #ff007f;
        }

        table {
            width: 100%;
            border-collapse: collapse;
        }

        th, td {
            padding: 10px;
            text-align: center;
            border: 1px solid #00ffcc;
        }

        th {
            background-color: #2a2a2a;
            color: #ff007f;
        }

        .current-user {
            background-color: rgba(255, 0, 127, 0.5); /* Highlight color for current user */
            font-weight: bold;
        }
    </style>
</head>
<body>
    <div class="nav-panel">
        <a href="home.php">Главная</a>
        <a href="logout.php">Выйти</a>
    </div>
    <div class="rating-container">
        <h1>Рейтинг пользователей</h1>
        
        <table>
            <thead>
                <tr>
                    <th>ID</th>
                    <th>Имя пользователя</th>
                    <th>Счет</th>
                </tr>
            </thead>
            <tbody>
                <?php

                foreach ($users as $user) {
                    $row_class = ($user['id'] === $user_info['id']) ? 'current-user' : '';
                    echo "<tr class='$row_class'>";
                    echo "<td>" . htmlspecialchars($user['id']) . "</td>";
                    echo "<td>" . htmlspecialchars($user['username']) . "</td>";
                    echo "<td>" . htmlspecialchars($user['score']) . "</td>";
                    echo "</tr>";
                }
                ?>
            </tbody>
        </table>
    </div>
</body>
</html>
