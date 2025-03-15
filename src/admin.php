<?php
require_once "config.php";
$conn = start_conn();

is_logged();

$user_info = get_user_info($conn);
if ($user_info['group_type'] !== 2) {
    header("Location: home.php");
    exit;
}

function get_sorted_users($conn) {
    $stmt = $conn->prepare("SELECT id, username, score, group_type, (SELECT COUNT(*) FROM tasks WHERE author_id = users.id) AS created_tasks_number FROM users ORDER BY score");
    $stmt->execute();
    $stmt->store_result();
    $stmt->bind_result($id, $username, $score, $group_type, $created_tasks_number);
    
    $result = [];
    
    while ($stmt->fetch()) {
        $result[] = ['id' => $id, 'username' => $username, 'score' => $score, 'group_type' => $group_type, 'created_tasks_number' => $created_tasks_number];
    }
    
    $stmt->close();
    
    return $result;
}

$users = get_sorted_users($conn);

function get_tasks($conn) {
    $tasks_query = "
        SELECT 
            t.id, 
            (SELECT name FROM categories WHERE id = t.category_id) AS category, 
            t.name, 
            t.readme, 
            t.status, 
            (SELECT username FROM users WHERE id = t.author_id) AS author, 
            t.author_id,
            -- Fetch hints as a JSON array
            (SELECT JSON_ARRAYAGG(JSON_OBJECT('hint', h.description, 'cost', h.cost)) 
             FROM hints h 
             WHERE h.task_id = t.id) AS hints
        FROM tasks t
    ";
    
    $tasks_result = $conn->query($tasks_query);

    $tasks_by_category = [];
    if ($tasks_result->num_rows > 0) {
        while ($row = $tasks_result->fetch_assoc()) {
            $tasks_by_category[$row['category']][] = [
                'id' => $row['id'],
                'name' => $row['name'],
                'status' => $row['status'],
                'readme' => $row['readme'],
                'author' => $row['author'],
                'author_id' => $row['author_id'],
                'hints' => json_decode($row['hints']) // Decode the JSON array of hints
            ];
        }
    }
    return $tasks_by_category;
}
function change_task_state($conn, $task_id, $new_status) {
    $stmt = $conn->prepare("UPDATE tasks SET status = ? WHERE id = ?");
    $stmt->bind_param("ii", $new_status, $task_id);
    $stmt->execute();
    $stmt->close();
    
    $stmt = $conn->prepare("SELECT name FROM tasks WHERE id = ?");
	$stmt->bind_param("i", $task_id);
	$stmt->execute();
	$stmt->store_result();
	$stmt->bind_result($task_name);
	$stmt->fetch();
	$stmt->close();
	
	$extractToPath = "/var/www/ctf_tasks/" . $task_name;
    $output_line = "";
    if ($new_status === 1)
    {
    	$command = 'docker-compose up -d --build && docker ps';
    }
    else
    {
    	$command = 'docker-compose down -v';
    }
    
    if ($new_status === 2)
    {
        $stmt = $conn->prepare("DELETE FROM tasks WHERE id = ?");
        $stmt->bind_param("i", $task_id);
        $stmt->execute();
        $stmt->close();
    }
    
    if (file_exists($extractToPath . "/" . "docker-compose.yml"))
    {
    	chdir($extractToPath);
        
		exec($command, $output, $return_var);

		if ($return_var === 0) {
			$output_line = "<p>Docker Compose used successfully in $extractToPath.</p><br>";
		} else {
			$output_line = "<p>Failed to use Docker Compose. Return code: $return_var.</p><br>";
		}
    }
    return $output_line;
}

$output_line = "";
if (isset($_POST['task_id']))
{
    $output_line = change_task_state($conn, intval($_POST['task_id']), intval($_POST['status']));
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
                        <th id="sortGroupType" style="cursor: pointer;">
							Кто ты, воин? 
							<span id="sortArrow" style="font-size: 0.8em; margin-left: 5px;"></span>
						</th>
                    </tr>
                </thead>
                <tbody id="userTableBody">
                    <?php foreach ($users as $user): ?>
                        <tr class="userRow">
                            <td><?php echo $user['id']; ?></td>
                            <td><a href="#" class="userLink" data-id="<?php echo $user['id']; ?>"><?php echo $user['username']; ?></a></td>
                            <td><?php echo $user['score']; ?></td>
                            <td><?php echo (($user['group_type'] === 0) ? "Пользователь" : (($user['group_type'] === 1) ? "Создатель" : "Админ")); ?></td>
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
                        <th>Автор</th>
                        <th>Статус</th>
                    </tr>
                </thead>
                <tbody id="taskTableBody">
                    <?php foreach ($tasks_by_category as $category => $tasks): ?>
                        <?php foreach ($tasks as $task): ?>
                            <tr class="taskRow">
                                <td><?php echo $category; ?></td>
                                <td>
                                    <a href="#" class="taskLink" 
                                       data-id="<?php echo $task['id']; ?>" 
                                       data-description="<?php echo htmlspecialchars($task['description']); ?>" 
                                       data-readme="<?php echo htmlspecialchars(base64_decode($task['readme'])); ?>" 
                                       data-flag="<?php echo htmlspecialchars($task['flag']); ?>" 
                                       data-hints='<?php echo json_encode($hints); ?>'> <!-- Assuming you fetch hints from the database -->
                                       <?php echo $task['name']; ?>
                                    </a>
                                </td>
                                <td><a href="#" class="userLink" data-id="<?php echo $task['author_id']; ?>"><?php echo $task['author']; ?></a></td>
                                <td>
                                    <form action="" method="POST" class="statusForm">
                                        <select name="status" class="taskStatus">
                                            <option value=1 <?php if ($task['status'] === "1") echo 'selected=""'; ?>>Одобрена</option>
                                            <option value=2 <?php if ($task['status'] === "2") echo 'selected=""'; ?>>Удалена</option>
                                            <option value=0 <?php if ($task['status'] === "0") echo 'selected=""'; ?>>Пока нету</option>
                                            <option value=3 <?php if ($task['status'] === "3") echo 'selected=""'; ?>>Отклонена</option>
                                        </select>
                                        <input type="hidden" name="task_id" value="<?php echo $task['id']; ?>">
                                        <button type="submit" class="applyButton">Применить</button>
                                    </form>
                                </td>
                            </tr>
                        <?php endforeach; ?>
                    <?php endforeach; ?>
                </tbody>
            </table>
            <?php echo $output_line; ?>
        </div>

        <script>
            document.getElementById('searchInput').addEventListener('input', function() {
                const query = this.value.toLowerCase();
                
                // Filter users
                const userRows = document.querySelectorAll('.userRow');
                userRows.forEach(row => {
                    const username = row.querySelector('td:nth-child(2) a').textContent.toLowerCase();
                    if (username.includes(query)) {
                        row.style.display = ''; // Show row if it matches
                    } else {
                        row.style.display = 'none'; // Hide row if it doesn't match
                    }
                });

                // Filter tasks
                const taskRows = document.querySelectorAll('.taskRow');
                taskRows.forEach(row => {
                    const taskName = row.querySelector('td:nth-child(2) a').textContent.toLowerCase();
                    if (taskName.includes(query)) {
                        row.style.display = ''; // Show row if it matches
                    } else {
                        row.style.display = 'none'; // Hide row if it doesn't match
                    }
                });
            });
        </script>

        <!-- Popup for Task Description -->
        <div class="overlay" id="overlay" style="display:none;"></div>
        <div class="popup" id="popup" style="display:none;">
            <h3>Описание задачи</h3>
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
        <script>
		    let currentSortOrder = 'ASC'; // Default sort order
			let clickCount = 0; // Count the number of clicks
			const sortArrow = document.getElementById('sortArrow');

			document.getElementById('sortGroupType').addEventListener('click', function() {
				const userTableBody = document.getElementById('userTableBody');
				const rows = Array.from(userTableBody.querySelectorAll('tr'));

				// Check the number of clicks to determine the action
				clickCount++;

				if (clickCount === 1) {
					// First click: Sort by group_type in ascending order
					rows.sort((a, b) => {
						const groupTypeA = getGroupTypeValue(a);
						const groupTypeB = getGroupTypeValue(b);
						return groupTypeA - groupTypeB; // Ascending order
					});
					sortArrow.textContent = '▲'; // Up arrow
				} else if (clickCount === 2) {
					// Second click: Sort by group_type in descending order
					rows.sort((a, b) => {
						const groupTypeA = getGroupTypeValue(a);
						const groupTypeB = getGroupTypeValue(b);
						return groupTypeB - groupTypeA; // Descending order
					});
					sortArrow.textContent = '▼'; // Down arrow
				} else {
					// Third click: Reset to default order (unsorted)
					clickCount = 0; // Reset click count
					rows.sort((a, b) => {
						return a.rowIndex - b.rowIndex; // Default order based on original index
					});
					sortArrow.textContent = ''; // No arrow
				}

				// Clear the table body and append sorted rows
				userTableBody.innerHTML = '';
				rows.forEach(row => userTableBody.appendChild(row));
			});

			// Helper function to get the group_type value from a row
			function getGroupTypeValue(row) {
				const groupTypeCell = row.querySelector('td:nth-child(4)'); // Adjust index if necessary
				const groupTypeText = groupTypeCell.textContent.trim();

				// Convert group type text to a comparable value
				switch (groupTypeText) {
					case 'Пользователь':
						return 0;
					case 'Создатель':
						return 1;
					case 'Админ':
						return 2;
					default:
						return 3; // Fallback for unknown types
				}
			}
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
                color: #00ffcc; /* Popup text color */
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
            
            /* Apply button styling */
            .applyButton {
                padding: 5px 10px;
                background-color: #007bff; /* Bootstrap primary color */
                color: white;
                border: none;
                border-radius: 5px;
                cursor: pointer;
                transition: background-color 0.3s;
            }

            .applyButton:hover {
                background-color: #0056b3; /* Darker shade on hover */
            }

            /* Style for task links */
            .taskLink {
                color: #00ffcc;
                text-decoration: underline;
                transition: color 0.3s;
            }

            .taskLink:hover {
                color: #ff007f; /* Change color on hover */
                text-decoration: none; /* Remove underline on hover */
            }

            /* Style for select dropdown */
            .taskStatus {
                padding: 5px;
                background-color: #1c1c1c;
                color: #00ffcc;
                border: 1px solid #00ffcc;
                border-radius: 5px;
                cursor: pointer;
                transition: background-color 0.3s;
            }

            .taskStatus:hover {
                background-color: #0056b3; /* Change background on hover */
            }

            .taskStatus option {
                background-color: #1c1c1c;
                color: #00ffcc;
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
        document.getElementById('popup').addEventListener('click', function(e) {
            if (e.target.id === 'saveTask') {
                const taskId = document.getElementById('taskId').value;
                const taskDescription = document.getElementById('taskDescription').value;
                const taskReadme = document.getElementById('taskReadme').value;
                const taskFlag = document.getElementById('taskFlag').value;

                // Gather hints
                const hints = Array.from(document.querySelectorAll('.hint')).map(hint => ({
                    text: hint.querySelector('.hintText').value,
                    cost: hint.querySelector('.hintCost').value
                }));

                // Send the updated task data to the server
                fetch('update_task.php', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        id: taskId,
                        description: taskDescription,
                        readme: btoa(unescape(encodeURIComponent(taskReadme))),
                        flag: taskFlag,
                        hints: hints,
                    }),
                })
                .then(response => response.json())
                .then(data => {
                    if (data.success) {
                        alert('Task updated successfully!');
                        location.reload(); // Reload the page to see changes
                    } else {
                        alert('Failed to update task.');
                    }
                })
                .catch((error) => {
                    console.error('Error:', error);
                });
            }
        });
        
        // Function to create the hint element
        function createHintElement(hint) {
            const hintDiv = document.createElement('div');
            hintDiv.classList.add('hint');
            console.log(hint);
            hintDiv.innerHTML = `
                <input type="text" class="hintText" value="${hint.hint_dscription}">
                <input type="number" class="hintCost" value="${hint.hint_cost}" min="0">
                <button class="deleteHint">Удалить</button>
            `;
            return hintDiv;
        }

        // Function to populate the popup with task details
        function populatePopup(task) {
            const { id, name, description, readme, flag, hints } = task;

            // Set popup content
            document.getElementById('popupContent').innerHTML = `
                <h4>${name}</h4>
                <p><strong>Описание:</strong> <textarea id="taskDescription">${description}</textarea></p>
                <p><strong>README:</strong> <textarea id="taskReadme">${readme}</textarea></p>
                <p><strong>Флаг:</strong> <input type="text" id="taskFlag" value="${flag}"></p>
                <div id="hintsContainer">
                    <h5>Подсказки:</h5>
                    ${hints.length > 0 ? hints.map(createHintElement).map(hintElement => hintElement.outerHTML).join('') : '<p>Нет подсказок.</p>'}
                    <button id="addHint">Добавить подсказку</button>
                </div>
                <input type="hidden" id="taskId" value="${id}">
                <button id="saveTask">Сохранить изменения</button>
            `;
            // console.log(hints);
        }

        // Function to show the popup
        function showPopup() {
            document.getElementById('popup').style.display = 'block';
            document.getElementById('overlay').style.display = 'block';
        }

        // Function to handle adding a new hint
        function handleAddHint() {
            const newHint = createHintElement({ hint: '', cost: 0 });
            document.getElementById('hintsContainer').appendChild(newHint);
        }

        // Function to handle deleting a hint
        function handleDeleteHint(event) {
            if (event.target.classList.contains('deleteHint')) {
                event.target.parentElement.remove();
            }
        }

        // Function to handle task link click
        function handleTaskLinkClick(event) {
            event.preventDefault();
            
            const taskId = this.getAttribute('data-id');

            // Fetch task details from the new PHP script
            fetch(`get_full_task_info.php?id=${taskId}`)
                .then(response => {
                    if (!response.ok) {
                        throw new Error('Network response was not ok');
                    }
                    return response.json();
                })
                .then(data => {
                    if (data.success) {
                        const task = {
                            id: taskId,
                            name: data.task.name,
                            description: data.task.description,
                            readme: decodeURIComponent(escape(atob(data.task.readme))),
                            flag: data.task.flag,
                            cost: data.task.cost,
                            first_blood_user: data.task.first_blood_user,
                            solutions_count: data.task.solutions_count,
                            hints: data.hints || [] // Default to an empty array if hints are null
                        };

                        populatePopup(task);
                        showPopup();

                        // Add event listeners
                        document.getElementById('addHint').addEventListener('click', handleAddHint);
                        document.getElementById('hintsContainer').addEventListener('click', handleDeleteHint);
                    } else {
                        alert(data.message); // Show error message if task not found
                    }
                })
                .catch(error => {
                    console.error('Error fetching task details:', error);
                    alert('An error occurred while fetching task details.');
                });
        }

        // Attach event listeners to task links
        document.querySelectorAll('.taskLink').forEach(link => {
            link.addEventListener('click', handleTaskLinkClick);
        });
        
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
                	if (user.group_type)
                		document.getElementById('popupContent').innerText = `ID: ${user.id}\nИмя пользователя: ${user.username}\nОчки: ${user.score}\nКол-во созданных задач: ${user.created_tasks_number}`;
                	else
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
