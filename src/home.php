<?php
require_once "config.php";
$conn = start_conn();

function get_categories($conn)
{
    $stmt = $conn->prepare("SELECT id, name, (SELECT COUNT(*) FROM tasks WHERE category_id = categories.id AND status = 1) AS amount FROM categories");
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
    $stmt = $conn->prepare("
        SELECT 
			id, 
			name, 
			category_id, 
			description, 
			level, 
			author_id, 
			cost - COALESCE((SELECT SUM(hints.cost) FROM hints WHERE task_id = tasks.id AND EXISTS (SELECT 1 FROM user_task_costs WHERE user_id = ? AND hint_id = hints.id)), 0) AS cost,
			hosting,
			files,
			flag,
			solution, 
			status, 
			readme
		FROM 
			tasks 
		WHERE 
			category_id = ? AND status = 1;
    ");
    $stmt->bind_param("ii", $_SESSION['id'], $category_id);
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
    
    return ($result !== [] && $result[0] === null) ? [] : $result[0];
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

is_logged();

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
    <title>CTF Панель</title>
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
    
    .hint-description-block {
        background-color: #000066; /* Light background color */
        border: 1px solid #ccc; /* Border for the block */
        border-radius: 5px; /* Rounded corners */
        padding: 15px; /* Padding inside the block */
        margin-top: 10px; /* Space above the block */
        box-shadow: 0 2px 5px rgba(0, 0, 0, 0.1); /* Subtle shadow effect */
    }

    .error-message {
        color: red; /* Red color for error messages */
        font-weight: bold; /* Bold text for emphasis */
    }
    
    #ratingForm {
		background-color: #2a2a2a; /* Dark background for the form */
		border: 1px solid #00ffcc; /* Border color matching the theme */
		border-radius: 5px; /* Rounded corners */
		padding: 15px; /* Padding for the form */
		margin-top: 20px; /* Space above the form */
		box-shadow: 0 4px 10px rgba(0, 255, 255, 0.2); /* Subtle shadow effect */
	}

	#ratingForm label {
		color: #00ffcc; /* Text color for the label */
		font-size: 18px; /* Font size for better visibility */
		margin-bottom: 10px; /* Space below the label */
		display: block; /* Make the label a block element */
	}

	#ratingInput {
		width: 100%; /* Full width for the select */
		padding: 10px; /* Padding for the select */
		border: 1px solid #00ffcc; /* Border color */
		border-radius: 5px; /* Rounded corners */
		background-color: #2a2a2a; /* Match background color */
		color: #00ffcc; /* Text color */
		margin-bottom: 15px; /* Space below the select */
		font-size: 16px; /* Font size for the select */
	}

	#ratingInput:focus {
		outline: none; /* Remove default outline */
		border-color: #ff007f; /* Change border color on focus */
		box-shadow: 0 0 5px rgba(255, 0, 127, 0.5); /* Add shadow on focus */
	}

	#ratingButton {
		background-color: #00ffcc; /* Button background color */
		color: #000; /* Button text color */
		padding: 10px 15px; /* Padding inside the button */
		border: none; /* Remove border */
		border-radius: 5px; /* Rounded corners for the button */
		cursor: pointer; /* Pointer cursor on hover */
		font-size: 16px; /* Font size for the button */
		transition: background 0.3s; /* Transition for hover effect */
	}

	#ratingButton:hover {
		background-color: #ff007f; /* Darker color on hover */
	}
    </style>
</head>
<body>
    <div class="sidebar">
        <h2>CTF Панель</h2>
        <a href="rating.php">Рейтинг</a>
        <a href="profile.php">Профиль</a>
        <a href="logout.php">Выйти</a>
        <?php if ($user_info['group_type'] !== 0) {echo '<a href="create.php">Создать задачу</a>';} ?>
        <?php if ($user_info['group_type'] === 2) {echo '<a href="admin.php">Админка</a>';} ?>
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
        <h1>Добро пожаловать в CTF Панель</h1>
        <?php
        $conn = start_conn();
        if ($_SERVER['REQUEST_METHOD'] === 'GET' && isset($_GET['category_id'])) {
            $tasks = get_category_tasks($conn, $_GET['category_id']);
            if (!empty($tasks)) {
                echo '<div class="task-list">';
                if ($solved_tasks === null) { $solved_tasks = []; }
                foreach ($tasks as $task) {
                	$_ = (in_array($task['id'], $solved_tasks)) ? "background-color: green;" : "";
                    echo '
                    <div class="task-item" style="border: 1px solid #ccc; padding: 10px; margin: 10px; cursor: pointer; ' . $_ . '" onclick="fetchTaskDetails(' . htmlspecialchars($task['id']) . ')">
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
                <p id="taskFileURLp"><strong>Ссылка на файлы для решения:</strong> <div class="tasks_url" id="taskFilesURL"></div></p>
                <p><strong>Количество решений:</strong> <span id="taskSolutionsCount"></span></p>
                <p><strong>Первая кровь:</strong> <span id="firstBloodUser"></span></p>
                <p><strong>Рейтинг задачи:</strong> <span id="taskRating"></span></p>
                <div id="hintSection" style="display:none;">
                    <strong>Подсказки:</strong>
                    <ul id="taskHintsList"></ul> <!-- List for hints -->
                </div>
                <div id="hintDescription" style="display:none;"></div> <!-- Section for hint description -->
                <form id="flagForm" onsubmit="submitFlag(event)">
                    <label for="flagInput">Введите флаг:</label>
                    <input type="text" id="flagInput" name="flag" required>
                    <button type="submit" class="flag_send">Отправить</button>
                </form>
                <form id="ratingForm" onsubmit="submitRating(event)" hidden>
					<input type="hidden" id="taskIdInput" name="task_id" required>
					<label for="ratingInput">Оцените задачу</label><br>
					<div>
						<select id="ratingInput" name="rating" required>
							<option value="" disabled selected>Выберите оценку</option>
							<option value="1">1</option>
							<option value="2">2</option>
							<option value="3">3</option>
							<option value="4">4</option>
							<option value="5">5</option>
						</select>
					</div>
					<button id='ratingButton' type="submit">Отправить оценку</button>
				</form>
            </div>
        </div>
    </div>
    
    <script>
		function submitRating(event) {
			event.preventDefault(); // Prevent default form submission
			const formData = new FormData(document.getElementById('ratingForm'));
			
			// Extract rating and task_id from the form data
			const rating = formData.get('rating');
			const taskId = formData.get('task_id');

			// Construct the URL with both rating and task_id
			const url = `submit_rating.php?rating=${rating}&task_id=${taskId}`;

			fetch(url)
			.then(response => response.text())
			.then(data => {
				// Create a notification element
				const notification = document.createElement('div');
				notification.className = 'notification';

				// Set the notification message based on the response
				notification.textContent = data.includes('success') ? 'Оценка успешно отправлена!' : 'Ошибка: ' + data;
				notification.style.backgroundColor = data.includes('success') ? 'green' : 'red'; // Set color based on response

				// Append notification to the body
				document.body.appendChild(notification);

				// Remove the notification after 5 seconds
				setTimeout(() => {
				    notification.remove();
				}, 5000);
			})
			.catch(error => {
				console.error('Error submitting rating:', error);
			});
		}
	</script>

    <script>
        function fetchTaskDetails(taskId) {
            fetch('get_task_details.php?id=' + taskId) // Fetch task details
                .then(response => response.json())
                .then(data => {
                    if (data.success) {
                        // Populate task details
                        document.getElementById('taskName').innerText = data.task.name;
                        document.getElementById('taskCost').innerText = data.task.cost;
                        document.getElementById('taskDescription').innerText = data.task.description;

                        // Set URLs if available
                        if (data.task.hosting) {
                            document.getElementById('taskURL').innerText = data.task.hosting;
                            document.getElementById('taskURL').href = data.task.hosting;
                        } else {
                            document.getElementById('taskURLp').hidden = true;
                        }
                        if (data.task.files) {
                            const allFiles = data.task.files.split(';');
                            const taskFilesURLDiv = document.getElementById('taskFilesURL');
                            taskFilesURLDiv.innerHTML = '';

                            allFiles.forEach(file => {
                                const anchor = document.createElement('a');
                                anchor.href = file;
                                anchor.innerText = file.split('/').pop();
                                anchor.target = '_blank';
                                taskFilesURLDiv.appendChild(anchor);
                                taskFilesURLDiv.appendChild(document.createElement('br'));
                            });
                        } else {
                            document.getElementById('taskFileURLp').hidden = true;
                        }

                        document.getElementById('taskSolutionsCount').innerText = data.task.solutions_count;
                        document.getElementById('firstBloodUser').innerText = data.task.first_blood_user;
                        document.getElementById('taskRating').innerText = data.task.task_rating;

                        // Fetch hints for the task
                        document.getElementById('taskHintsList').innerHTML = ''; // Clear previous hints
                        data.hints.forEach(hint => {
                            const li = document.createElement('li');
                            li.innerText = `Стоимость: ${hint.hint_cost}`; // Display hint cost only
                            li.dataset.hintId = hint.hint_id; // Store hint ID in data attribute
                            li.onclick = () => revealHint(hint.hint_id); // Add click event to fetch hint description
                            document.getElementById('taskHintsList').appendChild(li);
                        });
                        
                        if (data.isTaskSolved)
                        	document.getElementById('ratingForm').hidden = false;
                        else
                        	document.getElementById('ratingForm').hidden = true;
                        
                        document.getElementById('taskIdInput').value = taskId;
                        
                        document.getElementById('hintSection').style.display = 'block'; // Show hint section

                        // Show modal
                        document.getElementById('taskModal').style.display = 'flex';
                    }
                })
                .catch(error => {
                    console.error('Error fetching task details:', error);
                });
        }

        function revealHint(hintId) {
            fetch('reveal_hint.php?hint_id=' + hintId) // Fetch hint description
                .then(response => response.json())
                .then(data => {
                    const hintDescriptionDiv = document.getElementById('hintDescription');
                    hintDescriptionDiv.innerHTML = ''; // Clear previous hint description
                    const descriptionBlock = document.createElement('div'); // Create a new div for the description
                    descriptionBlock.className = 'hint-description-block'; // Add a class for styling

                    if (data.success) {
                        const p = document.createElement('p');
                        p.innerText = data.description; // Set hint description
                        descriptionBlock.appendChild(p); // Append description paragraph
                    } else {
                        console.error('Error fetching hint:', data.description);
                        const errorMessage = document.createElement('p');
                        errorMessage.innerText = data.description; // Display error message
                        errorMessage.className = 'error-message'; // Add a class for error styling
                        descriptionBlock.appendChild(errorMessage);
                    }

                    hintDescriptionDiv.appendChild(descriptionBlock); // Append the styled block to the hint description div
                    hintDescriptionDiv.style.display = 'block'; // Show hint description
                })
                .catch(error => {
                    console.error('Error fetching hint:', error);
                });
        }

        function closeModal() {
            document.getElementById('taskModal').style.display = 'none';
            document.getElementById('hintSection').style.display = 'none'; // Hide hint section when modal is closed
            document.getElementById('taskHintsList').innerHTML = ''; // Clear hints list
            document.getElementById('hintDescription').innerHTML = ''; // Clear hint description
            document.getElementById('hintDescription').style.display = 'none'; // Hide hint description
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
            .then(response => response.text())
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
