<?php
require_once "config.php";
$conn = start_conn();
session_start();

function get_categories($conn)
{
    $stmt = $conn->prepare("SELECT id, name, amount FROM categories");
    $stmt->execute();
    $stmt->store_result();
    $stmt->bind_result($id, $name, $amount);
    
    $result = [];
    
    while ($stmt->fetch())
    {
        $result[] = ['id' => $id, 'name' => $name, 'amount' => $amount];
    }
    
    $stmt->close();
    
    return $result;
}

function get_category_tasks($conn, $category_id)
{
    $stmt = $conn->prepare("SELECT * FROM tasks where category_id = ?");
    $stmt->bind_param("i", $category_id);
    $stmt->execute();
    $result = $stmt->get_result();
    
    $tasks = [];
    
    while ($row = $result->fetch_assoc()) {
        $tasks[] = $row;
    }
    
    $stmt->close();
    
    return $tasks;
}

function get_solved_tasks($conn)
{
    $stmt = $conn->prepare("SELECT task_id FROM solved_tasks WHERE user_id = ?");
    $stmt->bind_param("i", $_SESSION['id']);
    $stmt->execute();
    $stmt->store_result();
    $stmt->bind_result($id);
    
    $result = [];
    
    while ($stmt->fetch())
    {
        $result[] = ['id' => $id];
    }
    
    $stmt->close();
    
    return $result[0];
}

function get_amount_solved_tasks_by_user($conn) {
    $query = "
        SELECT c.name AS category_name, COUNT(st.task_id) AS solved_tasks_count
        FROM categories c
        LEFT JOIN tasks t ON c.id = t.category_id
        LEFT JOIN solved_tasks st ON t.id = st.task_id AND st.user_id = ?
        GROUP BY c.id;
    ";
    $stmt = $conn->prepare($query);
    $stmt->bind_param("i", $_SESSION['id']);

    $solvedTasksByCategory = [];
    
    if ($stmt) {
        $stmt->execute();
        $stmt->bind_result($category_name, $solved_tasks_count);
        while ($stmt->fetch()) {
            $solvedTasksByCategory[$category_name] = (int)$solved_tasks_count;
        }

        $stmt->close();
    }

    return $solvedTasksByCategory;
}

if (!is_logged())
{
    header("Location: login.php");
    exit;
}

# okay, now very hard. I need index.html, where we need a panel, where will be links to rating.php, profile.php and tasks.php. Also, there will be a list of categories (which will be got by php). Do it in cyberpunk style

$solved_tasks = get_solved_tasks($conn);
$user_info = get_user_info($conn);
$categories = get_categories($conn);
$category_solved = get_amount_solved_tasks_by_user($conn);
$conn->close();

?>
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Киберпанк Панель</title>
    <style>
        body {
            font-family: 'Courier New', Courier, monospace;
            background-color: #121212; /* Темный фон */
            color: #00ffcc; /* Нейронный текст */
            display: flex;
            height: 100vh;
            margin: 0;
        }
        /* Стили для боковой панели */
        .sidebar {
            width: 250px;
            background-color: #1c1c1c; /* Темная боковая панель */
            padding: 20px;
            box-shadow: 0 0 20px rgba(0, 255, 255, 0.3);
        }
        .sidebar h2 {
            text-align: center;
            color: #ff007f; /* Нейронный розовый */
            text-shadow: 0 0 20px rgba(255, 0, 127, 0.5);
            margin-bottom: 20px;
        }
        .sidebar a {
            display: block;
            color: #00ffcc; /* Нейронный текст */
            text-decoration: none;
            padding: 10px;
            margin: 5px 0;
            border-radius: 5px;
            transition: background 0.3s, color 0.3s;
        }
        a.tasks_url {
        	color: white;
        }
        .sidebar a:hover {
            background-color: rgba(0, 255, 255, 0.2);
            color: #ff007f; /* Изменение цвета текста при наведении */
        }
        /* Стили для категорий */
        .categories {
            margin-top: 20px;
        }
        .categories h3 {
            color: #ff007f; /* Нейронный розовый */
            text-shadow: 0 0 10px rgba(255, 0, 127, 0.5);
        }
        /* Стили для основного контента */
        .main-content {
            flex-grow: 1;
            padding: 20px;
            background-color: #181818; /* Темный основной контент */
            color: #fff; /* Белый текст */
            box-shadow: inset 0 0 10px rgba(255, 255, 255, 0.1);
        }
        
        /* Styles for the form */
        .category-item {
            background-color: #2a2a2a; /* Dark background for category items */
            color: #00ffcc; /* Neon text color */
            border: none; /* Remove default button border */
            padding: 10px; /* Add padding for better click area */
            border-radius: 5px; /* Rounded corners */
            cursor: pointer; /* Change cursor to pointer on hover */
            transition: background 0.3s, color 0.3s; /* Smooth transition for hover effects */
            box-shadow: 0 0 10px rgba(0, 255, 255, 0.3);
            display: block; /* Ensure button takes full width */
            width: 100%; /* Full width for button */
        }
        /* Change background color on hover */
        .category-item:hover {
            background-color: rgba(0, 255, 255, 0.3); /* Lighten background on hover */
            color: #ff007f; /* Change text color on hover */
        }
        /* Hide the default input */
        input[type="hidden"] {
            display: none; /* Hide the hidden input field */
        }
        
        form.category {
            display: block; /* Each form takes a full line */
            margin-bottom: 10px; /* Space between forms */
        }
        /* Адаптивные стили */
        @media (max-width: 768px) {
            body {
                flex-direction: column;
            }
            .sidebar {
                width: 100%;
                height: auto;
            }
            .main-content {
                padding: 10px;
            }
        }
        
        .task-list {
            display: flex;
            flex-wrap: wrap;
            gap: 20px; /* Space between task items */
            margin-top: 20px;
        }
        .task-item {
            background-color: #2a2a2a; /* Dark background for task boxes */
            color: #00ffcc; /* Text color */
            padding: 15px;
            border-radius: 10px;
            box-shadow: 0 0 10px rgba(0, 255, 255, 0.3);
            flex: 1 1 calc(30% - 20px); /* Responsive width */
            min-width: 250px; /* Minimum width for task boxes */
            cursor: pointer; /* Pointer cursor on hover */
        }
        .task-item h2 {
            margin: 0 0 10px; /* Margin for task title */
        }
        .task-item p {
            margin: 0; /* Remove default margin for paragraph */
        }
        .modal {
            display: flex;
            position: fixed; /* Stay in place */
            z-index: 1; /* Sit on top */
            left: 0;
            top: 0;
            width: 100%; /* Full width */
            height: 100%; /* Full height */
            overflow: auto; /* Enable scroll if needed */
            background-color: rgba(0, 0, 0, 0.7); /* Black with opacity */
        }
        .modal-content {
            background-color: #1c1c1c;
            margin: auto;
            padding: 20px;
            border: 1px solid #00ffcc;
            border-radius: 10px;
            width: 80%; /* Could be more or less, depending on screen size */
            max-width: 600px; /* Maximum width */
            color: #00ffcc; /* Text color */
        }
        .close {
            color: #ff007f;
            float: right;
            font-size: 28px;
            font-weight: bold;
            cursor: pointer;
        }
        .close:hover,
        .close:focus {
            color: #ff007f;
            text-decoration: none;
            cursor: pointer;
        }
        
        /* Existing modal styles */
    .modal {
        display: flex;
        position: fixed; /* Stay in place */
        z-index: 1; /* Sit on top */
        left: 0;
        top: 0;
        width: 100%; /* Full width */
        height: 100%; /* Full height */
        overflow: auto; /* Enable scroll if needed */
        background-color: rgba(0, 0, 0, 0.7); /* Black with opacity */
    }

    .modal-content {
        background-color: #1c1c1c;
        margin: auto;
        padding: 20px;
        border: 1px solid #00ffcc;
        border-radius: 10px;
        width: 80%; /* Could be more or less, depending on screen size */
        max-width: 600px; /* Maximum width */
        color: #00ffcc; /* Text color */
    }

    .close {
        color: #ff007f;
        float: right;
        font-size: 28px;
        font-weight: bold;
        cursor: pointer;
    }

    .close:hover,
    .close:focus {
        color: #ff007f;
        text-decoration: none;
        cursor: pointer;
    }

    /* Flag input styles */
    #flagForm {
        margin-top: 20px;
    }

    #flagInput {
        width: 100%; /* Full width */
        padding: 10px;
        margin: 10px 0;
        border: 1px solid #00ffcc;
        border-radius: 5px;
        background-color: #2a2a2a;
        color: #00ffcc;
    }

    button[type="submit"].flag_send {
        background-color: #00ffcc;
        color: #000;
        border: none;
        border-radius: 5px;
        padding: 10px 20px;
        font-size: 16px;
        cursor: pointer;
        transition: background 0.3s;
    }

    button[type="submit"].flag_send:hover {
        background-color: #ff007f; /* Change color on hover */
    }
    
    .notification {
        position: fixed;
        top: 20px;
        left: 50%;
        transform: translateX(-50%);
        padding: 10px 20px;
        border-radius: 5px;
        color: white;
        font-size: 16px;
        z-index: 1000; /* Ensure it appears above other content */
        transition: opacity 0.5s ease; /* Optional: Fade effect */
    }
    </style>
</head>
<body>
    <div class="sidebar">
        <h2>Киберпанк Панель</h2>
        <a href="rating.php">Рейтинг</a>
        <a href="profile.php">Профиль</a>
        <a href="logout.php">Выйти</a>
        <?php if ($user_info['group_type'] !== 0) {echo '<a href="create.php">Создать задачу</a>';} ?>
        <div class="categories">
            <h3>Категории</h3>
            <?php
            foreach ($categories as $category) {
                echo '<form class="category" method="GET" action="">';
                echo '<input type="hidden" name="category_id" value="' . $category['id'] . '">';
                echo '<button type="submit" class="category-item">' . htmlspecialchars($category['name']) . '   ' . $category_solved[$category['name']] . '/' . $category['amount'] . '</button>';
                echo '</form>';
            }
            ?>
        </div>
    </div>
    <div class="main-content">
        <h1>Добро пожаловать в Киберпанк Панель</h1>
        <?php
        $conn = start_conn();
        if ($_SERVER['REQUEST_METHOD'] === 'GET' && isset($_GET['category_id'])) {
            $tasks = get_category_tasks($conn, $_GET['category_id']);
            if (!empty($tasks)) {
                echo '<div class="task-list">';
                foreach ($tasks as $task) {
                    echo '
                    <div class="task-item" ' . ( empty($solved_tasks) ? '' : (( in_array($task['id'], $solved_tasks) ) ? 'style="background-color: green"' : '')) . ' onclick="showTaskDetails(' . htmlspecialchars($task['id']) . ')">
                        <h2>' . htmlspecialchars($task['name']) . '</h2>
                        <p>Стоимость: <strong>' . htmlspecialchars($task['cost']) . '</strong></p>
                    </div>';
                }
                echo '</div>';
            }
        }
        
        $conn->close();
        ?>
        
        <div id="taskModal" class="modal" style="display:none;">
            <div class="modal-content">
                <span class="close" onclick="closeModal()">&times;</span>
                <h2 id="taskName"></h2>
                <p><strong>Стоимость:</strong> <span id="taskCost"></span></p>
                <p><strong>Описание:</strong> <span id="taskDescription"></span></p>
                <p id="taskURLp"><strong>Ссылка:</strong> <a class="tasks_url" id="taskURL"></a></p>
                <p id="taskFileURLp"><strong>Ссылка на файлы для решения:</strong> <a class="tasks_url" id="taskFileURL"></a></p>
                <p><strong>Количество решений:</strong> <span id="taskSolutionsCount"></span></p>
                <p><strong>Первая кровь:</strong> <span id="firstBloodUser"></span></p>
                <form id="flagForm" onsubmit="submitFlag(event)">
                    <label for="flagInput">Введите флаг:</label>
                    <input type="text" id="flagInput" name="flag" required>
                    <button type="submit" class="flag_send">Отправить</button>
                </form>
            </div>
        </div>
    </div>
    <script>
        function showTaskDetails(taskId) {
            // Fetch task details using AJAX
            fetch('get_task_details.php?id=' + taskId)
                .then(response => response.json())
                .then(data => {
                    if (data.success) {
                        // Populate task details
                        document.getElementById('taskName').innerText = data.task.name;
                        document.getElementById('taskCost').innerText = data.task.cost;
                        document.getElementById('taskDescription').innerText = data.task.description;
                        if (data.task.hosting !== null)
                        {
		                    document.getElementById('taskURL').innerText = data.task.hosting;
		                    document.getElementById('taskURL').href = data.task.hosting;
		                }
                        else
                        {
                        	document.getElementById('taskURLp').hidden = true;
                        }
                        if (data.task.files !== null)
                        {
                        	document.getElementById('taskFileURL').innerText = data.task.files.split('/')[3];
                        	document.getElementById('taskFileURL').href = data.task.files;
                        }
                        else
                        {
                        	document.getElementById('taskFileURLp').hidden = true;
                        }
                        document.getElementById('taskSolutionsCount').innerText = data.task.solutions_count;
                        document.getElementById('firstBloodUser').innerText = data.task.first_blood_user;
                        // Show modal
                        document.getElementById('taskModal').style.display = 'flex';
                    }
                })
                .catch(error => {
                    console.error('Error fetching task details:', error);
                });
        }
        
        function closeModal() {
            document.getElementById('taskModal').style.display = 'none';
        }
        
        function submitFlag(event) {
            event.preventDefault(); // Prevent form submission
            const flag = document.getElementById('flagInput').value;
            const taskName = document.getElementById('taskName').innerText; // Get the task name from the modal

            // Create a FormData object to hold the flag and task name
            const formData = new FormData();
            formData.append('flag', flag);
            formData.append('task', taskName); // Append task name to FormData

            // Submit flag via AJAX
            fetch('submit_flag.php', {
                method: 'POST',
                body: formData, // Send the FormData object directly
            })
            .then(response => response.text()) // Change to text response
            .then(data => {
                // Create a notification element
                const notification = document.createElement('div');
                notification.className = 'notification';

                // Set the notification message and color based on the response
                if (data.includes('success')) {
                    notification.textContent = 'Флаг успешно отправлен!';
                    notification.style.backgroundColor = 'green'; // Success color
                    closeModal(); // Close modal after submission
                    location.reload();
                } else {
                    notification.textContent = 'Ошибка: ' + data;
                    notification.style.backgroundColor = 'red'; // Error color
                }

                // Append notification to the body
                document.body.appendChild(notification);

                // Remove the notification after 5 seconds
                setTimeout(() => {
                    notification.remove();
                }, 5000);
            })
            .catch(error => {
                console.error('Error submitting flag:', error);
            });
        }
    </script>
</body>
</html>
