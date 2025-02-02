<?php
require_once "config.php"; // Include your database configuration
$conn = start_conn();
session_start();

is_logged();

$user_info = get_user_info($conn);
if ($user_info['group_type'] !== 1) {
    header("Location: home.php");
    exit;
}

function get_sorted_users($conn) {
    $stmt = $conn->prepare("SELECT id, username, score FROM users ORDER BY score");
    $stmt->execute();
    $stmt->store_result();
    $stmt->bind_result($id, $username, $score);
    
    $result = [];
    
    while ($stmt->fetch()) {
        $result[] = ['id' => $id, 'username' => $username, 'score' => $score];
    }
    
    $stmt->close();
    
    return $result;
}

$users = get_sorted_users($conn);

function get_tasks($conn) {
    $tasks_query = "SELECT (SELECT name FROM categories WHERE id = category_id) AS category, name, readme, status FROM tasks";
    $tasks_result = $conn->query($tasks_query);

    $tasks_by_category = [];
    if ($tasks_result->num_rows > 0) {
        while ($row = $tasks_result->fetch_assoc()) {
            $tasks_by_category[$row['category']][] = [
                'name' => $row['name'],
                'status' => $row['status'],
                'readme' => $row['readme']
            ]; // Group tasks by category
        }
    }
    return $tasks_by_category;
}

$tasks_by_category = get_tasks($conn);
$conn->close();
?>

<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Админ Панель</title>
    <style>
        body {
            display: flex;
            font-family: Arial, sans-serif;
            background-color: #121212;
            color: #00ffcc;
            height: 100vh;
            margin: 0;
        }

        /* Navigation panel styling */
        .nav-panel {
            background-color: #1c1c1c;
            padding: 10px;
            box-shadow: 0 2px 10px rgba(0, 0, 0, 0.5);
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            z-index: 1000;
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

        /* Sidebar styling */
        .sidebar {
            width: 250px;
            background-color: #1c1c1c;
            padding: 20px;
            box-shadow: 2px 0 5px rgba(0,0,0,0.5);
            margin-top: 50px; /* Space for the nav-panel */
        }

        .sidebar h2 {
            margin-bottom: 20px;
        }

        .sidebar a {
            color: #00ffcc;
            text-decoration: none;
            display: block;
            margin: 10px 0;
            padding: 10px;
            border-radius: 5px;
            transition: background-color 0.3s;
        }

        .sidebar a:hover {
            background-color: #0056b3;
        }

        /* Main content styling */
        .main-content {
            flex-grow: 1;
            padding: 20px;
            background-color: #2a2a2a;
            position: relative;
            margin-top: 50px; /* Space for the nav-panel */
        }

        /* User List styling */
        .user-list, .task-list {
            display: none; /* Hide by default */
        }

        .user-list table, .task-list table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 20px;
        }

        .user-list th, .task-list th, .user-list td, .task-list td {
            border: 1px solid #00ffcc;
            padding: 10px;
            text-align: left;
        }

        /* Search bar styling */
        .search-bar {
            margin: 20px 0;
        }

        .search-bar input {
            padding: 10px;
            width: 100%;
            border: 1px solid #00ffcc;
            border-radius: 5px;
            background-color: #1c1c1c;
            color: #fff;
        }

        /* Popup styling */
        .popup {
            display: none; /* Hidden by default */
            position: fixed;
            left: 50%;
            top: 50%;
            transform: translate(-50%, -50%);
            background-color: #1c1c1c;
            padding: 20px;
            border-radius: 10px;
            box-shadow: 0 0 15px rgba(0, 0, 0, 0.5);
            z-index: 1000;
        }

        .popup h3 {
            margin: 0 0 10px 0;
        }

        .popup button {
            margin-top: 10px;
            padding: 10px;
            background-color: #ff007f;
            color: white;
            border: none;
            border-radius: 5px;
            cursor: pointer;
        }

        .popup button:hover {
            background-color: #e6007a;
        }

        /* Overlay for popup */
        .overlay {
            display: none; /* Hidden by default */
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0, 0, 0, 0.7);
            z-index: 999;
        }
    </style>
</head>
<body>
    <div class="nav-panel">
        <a href="home.php">Главная</a>
        <a href="logout.php">Выйти</a>
    </div>
    <div class="sidebar">
        <h2>Админ Панель</h2>
        <a href="#" id="viewUsers">Посмотреть пользователей</a>
        <a href="#" id="viewTasks">Посмотреть задачи</a>
    </div>

    <div class="main-content">
        <div class="search-bar">
            <input type="text" id="searchInput" placeholder="Поиск...">
        </div>

        <div class="user-list" id="userList">
            <h3>Список пользователей</h3>
            <table>
                <thead>
                    <tr>
                        <th>ID</th>
                        <th>Имя пользователя</th>
                        <th>Очки</th>
                    </tr>
                </thead>
                <tbody id="userTableBody">
                    <?php foreach ($users as $user): ?>
                        <tr>
                            <td><?php echo $user['id']; ?></td>
                            <td><a href="#" class="userLink" data-id="<?php echo $user['id']; ?>"><?php echo $user['username']; ?></a></td>
                            <td><?php echo $user['score']; ?></td>
                        </tr>
                    <?php endforeach; ?>
                </tbody>
            </table>
        </div>

        <div class="task-list" id="taskList">
            <h3>Список задач</h3>
            <table>
                <thead>
                    <tr>
                        <th>Категория</th>
                        <th>Задачи</th>
                        <th>Статус</th>
                    </tr>
                </thead>
                <tbody id="taskTableBody">
                    <?php foreach ($tasks_by_category as $category => $tasks): ?>
                        <?php foreach ($tasks as $task): ?>
                            <tr>
                                <td><?php echo $category; ?></td>
                                <td>
                                    <a href="#" class="taskLink" data-description="<?php echo htmlspecialchars(base64_decode($task['readme'])); ?>">
                                        <?php echo $task['name']; ?>
                                    </a>
                                </td>
                                <td>
                                    <select class="taskStatus">
                                        <option value="on" <?php if ($task['status'] === 'on') echo 'selected'; ?>>Одобрена</option>
                                        <option value="deleted" <?php if ($task['status'] === 'deleted') echo 'selected'; ?>>Удалена</option>
                                        <option value="off" <?php if ($task['status'] === 'off') echo 'selected'; ?>>Пока нету</option>
                                        <option value="declined" <?php if ($task['status'] === 'declined') echo 'selected'; ?>>Отклонена</option>
                                    </select>
                                </td>
                            </tr>
                        <?php endforeach; ?>
                    <?php endforeach; ?>
                </tbody>
            </table>
        </div>

        <!-- Popup for Task Description -->
        <div class="overlay" id="overlay" style="display:none;"></div>
        <div class="popup" id="popup" style="display:none;">
            <h3>README задачи</h3>
            <p id="popupContent"></p>
            <button id="closePopup">Закрыть</button>
        </div>

        <script>
            document.querySelectorAll('.taskLink').forEach(link => {
                link.addEventListener('click', function(e) {
                    e.preventDefault();
                    const description = this.getAttribute('data-description');

                    // Set popup content
                    document.getElementById('popupContent').innerText = description;

                    // Show popup and overlay
                    document.getElementById('popup').style.display = 'block';
                    document.getElementById('overlay').style.display = 'block';
                });
            });

            // Close popup
            document.getElementById('closePopup').addEventListener('click', function() {
                document.getElementById('popup').style.display = 'none';
                document.getElementById('overlay').style.display = 'none';
            });

            // Close overlay on click
            document.getElementById('overlay').addEventListener('click', function() {
                document.getElementById('popup').style.display = 'none';
                document.getElementById('overlay').style.display = 'none';
            });
        </script>

        <style>
            /* Popup styling */
            .popup {
                display: none; /* Hidden by default */
                position: fixed;
                left: 50%;
                top: 50%;
                transform: translate(-50%, -50%);
                background-color: #1c1c1c;
                padding: 20px;
                border-radius: 10px;
                box-shadow: 0 0 15px rgba(0, 0, 0, 0.5);
                z-index: 1000;
            }

            /* Overlay for popup */
            .overlay {
                display: none; /* Hidden by default */
                position: fixed;
                top: 0;
                left: 0;
                width: 100%;
                height: 100%;
                background: rgba(0, 0, 0, 0.7);
                z-index: 999;
            }
        </style>

        <div class="overlay" id="overlay"></div>
        <div class="popup" id="popup">
            <h3>Информация о пользователе</h3>
            <p id="popupContent"></p>
            <button id="closePopup">Закрыть</button>
        </div>
    </div>

    <script>
        // Event listeners
        document.getElementById('viewUsers').addEventListener('click', function() {
            document.getElementById('userList').style.display = 'block';
            document.getElementById('taskList').style.display = 'none';
        });
        
        document.getElementById('viewTasks').addEventListener('click', function() {
            document.getElementById('taskList').style.display = 'block';
            document.getElementById('userList').style.display = 'none';
        });

        // Handle user link click
        document.addEventListener('click', function(e) {
            if (e.target.classList.contains('userLink')) {
                e.preventDefault();
                const userId = e.target.getAttribute('data-id');
                const user = <?php echo json_encode($users); ?>.find(u => u.id == userId);
                if (user) {
                    document.getElementById('popupContent').innerText = `ID: ${user.id}\nИмя пользователя: ${user.username}\nОчки: ${user.score}`;
                    document.getElementById('popup').style.display = 'block';
                    document.getElementById('overlay').style.display = 'block';
                }
            }
        });

        // Close popup
        document.getElementById('closePopup').addEventListener('click', function() {
            document.getElementById('popup').style.display = 'none';
            document.getElementById('overlay').style.display = 'none';
        });

        // Close overlay on click
        document.getElementById('overlay').addEventListener('click', function() {
            document.getElementById('popup').style.display = 'none';
            document.getElementById('overlay').style.display = 'none';
        });

        // Initialize with users displayed
        document.getElementById('viewUsers').click();
    </script>
</body>
</html>
