<?php
require_once "config.php";

$conn = start_conn();
session_start();

if (!is_logged())
{
    header("Location: login.php");
    exit;
}

$user_info = get_user_info($conn);
if ($user_info['group_type'] === 0)
{
	header("Location: home.php");
	exit;
}

function get_completed_tasks($conn)
{
	$stmt = $conn->prepare("SELECT task_id FROM solved_tasks WHERE user_id = ?");
	$stmt->bind_param("i", $_SESSION['id']);
    $stmt->execute();
    $stmt->store_result();
    $stmt->bind_result($task_id);
    
    $result = [];
    
    while ($stmt->fetch())
    {
    	$result[] = ["id" => $task_id];
    }
    $stmt->close();
    
    return $result[0];
}

function get_number_fbs($conn)
{
	$stmt = $conn->prepare("SELECT COUNT(*) FROM solved_tasks WHERE user_id = ?");
	$stmt->bind_param("i", $_SESSION['id']);
    $stmt->execute();
    $stmt->store_result();
    $stmt->bind_result($count);
    
    $stmt->fetch();
    $stmt->close();
    
    return $count;
}

function get_rank_place($conn)
{
	$stmt = $conn->prepare("SELECT id FROM users WHERE username = (SELECT username FROM users WHERE id = ?) ORDER BY score");
	$stmt->bind_param("i", $_SESSION['id']);
    $stmt->execute();
    $stmt->store_result();
    $stmt->bind_result($id);
    
    $stmt->fetch();
    $stmt->close();
    
    return $id;
}

$completed_tasks = get_completed_tasks($conn);
$completed_tasks = ($completed_tasks === null) ? [] : $completed_tasks;
$place = get_rank_place($conn);
$fbs = get_number_fbs($conn);
$conn->close();
?>

<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Профиль</title>
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

        .profile-container {
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

        .profile-image-container {
            position: relative;
            display: flex;
            justify-content: center; /* Center the image horizontally */
            margin: 0 auto 20px; /* Center the container */
        }

        .profile-image {
            border-radius: 50%;
            width: 150px;
            height: 150px;
            object-fit: cover;
            cursor: pointer; /* Change cursor to pointer */
        }

        .upload-image-form {
            position: absolute;
            bottom: 10px; /* Position above the bottom */
            right: 10px; /* Position to the right */
        }

        .upload-image {
            background-color: rgba(0, 255, 255, 0.8); /* Semi-transparent background for visibility */
            color: #000;
            border: none;
            border-radius: 5px;
            padding: 5px 10px;
            cursor: pointer;
            transition: background 0.3s;
            font-size: 12px; /* Smaller font size */
            text-align: center; /* Center text */
        }

        .upload-image:hover {
            background-color: rgba(0, 255, 255, 1); /* Full opacity on hover */
        }

        .statistic {
            margin: 10px 0;
        }

        .task-category {
            margin-top: 20px;
        }

        .task-category h3 {
            color: #ff007f;
        }

        .upload-image-form input {
            display: none; /* Hide the file input */
        }
    </style>
</head>
<body>
    <div class="nav-panel">
        <a href="home.php">Главная</a>
        <a href="logout.php">Выйти</a>
    </div>

    <div class="profile-container">
        <h1>Профиль пользователя</h1>
        
        <div class="profile-image-container">
            <img src="<?php echo htmlspecialchars('profiles/'. $user_info['id'] . '.jpg'); ?>" alt="Profile Image" class="profile-image" id="profileImage">
            <input type="file" name="profile_image" id="profile_image" accept="image/*" style="display: none;" onchange="uploadImage()">
        </div>

        <div class="statistic">
            <strong>Имя пользователя:</strong> <?php echo htmlspecialchars($user_info['username']); ?><br>
            <strong>Счет:</strong> <?php echo htmlspecialchars($user_info['score']); ?><br>
            <strong>Количество решенных задач:</strong> <?php echo htmlspecialchars(count($completed_tasks)); ?><br>
            <strong>Место в рейтинге:</strong> <?php echo htmlspecialchars($place); ?><br>
            <strong>Первая кровь:</strong> <?php echo htmlspecialchars($fbs); ?>
        </div>
    </div>

    <script>
        // Make the image clickable to trigger file input
        document.getElementById('profileImage').addEventListener('click', function() {
            document.getElementById('profile_image').click();
        });

        function uploadImage() {
            const fileInput = document.getElementById('profile_image');
            const file = fileInput.files[0];
            const userId = <?php echo json_encode($user_info['id']); ?>; // Get user ID from PHP

            if (file) {
                const formData = new FormData();
                formData.append('profile_image', file);
                formData.append('user_id', userId); // Append user ID

                fetch('upload_image.php', {
                    method: 'POST',
                    body: formData
                })
                .then(response => response.json())
                .then(data => {
                    const notification = document.getElementById('notification');

                    if (data.success) {
                        // Update the profile image on success
                        document.getElementById('profileImage').src = `profiles/${userId}.jpg?${new Date().getTime()}`; // Cache-busting
                        notification.textContent = data.message; // Show success message
                    } else {
                        notification.textContent = data.message; // Show error message
                    }

                    notification.style.display = 'block'; // Show notification
                    notification.style.opacity = 1; // Set opacity to 1 for fade-in effect

                    // Fade out the notification after 5 seconds
                    setTimeout(() => {
                        notification.style.opacity = 0; // Fade out
                        setTimeout(() => {
                            notification.style.display = 'none'; // Hide after fade out
                        }, 500); // Wait for fade out to finish
                    }, 5000); // Display for 5 seconds
                })
                .catch(error => {
                    console.error('Error:', error);
                    const notification = document.getElementById('notification');
                    notification.textContent = 'An error occurred while uploading the image.';
                    notification.style.display = 'block'; // Show notification
                    notification.style.opacity = 1; // Set opacity to 1 for fade-in effect

                    // Fade out the notification after 5 seconds
                    setTimeout(() => {
                        notification.style.opacity = 0; // Fade out
                        setTimeout(() => {
                            notification.style.display = 'none'; // Hide after fade out
                        }, 500); // Wait for fade out to finish
                    }, 5000); // Display for 5 seconds
                });
            }
        }
    </script>
</body>
</html>
