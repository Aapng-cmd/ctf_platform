<?php
require_once "config.php";

$conn = start_conn();

function get_users_count($conn) {
    $sql = "SELECT COUNT(*) AS user_count FROM users";
    $stmt = $conn->prepare($sql);
    $stmt->execute();
    $stmt->bind_result($user_count);
    $stmt->fetch();
    $stmt->close();
    return $user_count;
}

function get_tasks_count($conn) {
    $sql = "SELECT COUNT(*) AS task_count FROM tasks WHERE status = 1";
    $stmt = $conn->prepare($sql);
    $stmt->execute();
    $stmt->bind_result($task_count);
    $stmt->fetch();
    $stmt->close();
    return $task_count;
}

$users_count = get_users_count($conn);
$tasks_count = get_tasks_count($conn);

$conn->close();

?>

<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <script src="js/nextparticle.min.js"></script>
    <link href="https://fonts.googleapis.com/css2?family=Pirata+One&amp;display=swap" rel="stylesheet">
    <title>CTF Платформа</title>
    <style>
        body {
            background-color: #121212; /* Dark background */
            color: #00ffcc; /* Neon green text */
            font-family: 'Courier New', Courier, monospace; /* Monospace font */
            display: flex;
            flex-direction: column;
            align-items: center;
            height: 100vh;
            margin: 0;
            padding: 20px;
        }

        .nav-panel {
            background-color: #1c1c1c;
            padding: 10px;
            text-align: center;
            border-radius: 5px;
            width: 100%;
            position: fixed;
            top: 0;
            left: 0;
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

        .main-container {
            text-align: center;
            margin-top: 60px; /* Space for the fixed navbar */
        }

        h1 {
            margin-bottom: 20px;
        }

        .bar-container {
            width: 80%; /* Width of the bar container */
            margin: 20px auto; /* Centering the container */
        }

        .bar {
            background-color: #2a2a2a;
            border-radius: 10px;
            margin: 10px 0;
            height: 30px; /* Height of the bars */
            position: relative; /* Positioning for label */
        }

        .bar-fill {
            background-color: #007bff; /* Blue bar fill */
            height: 100%; /* Fill the height of the bar */
            border-radius: 10px; /* Rounded corners for the fill */
            transition: width 0.5s ease; /* Smooth transition for width change */
        }

        .bar-label {
            position: absolute;
            left: 50%; /* Centering the label */
            transform: translateX(-50%); /* Adjusting for perfect center */
            color: #fff; /* Default text color */
            font-weight: bold; /* Bold label text */
        }

        .info-section {
            background-color: #1c1c1c;
            border-radius: 5px;
            padding: 20px;
            margin: 20px auto;
            width: 80%; /* Width of the info section */
            color: #ffffff; /* White text for contrast */
        }

        .info-section h2 {
            margin-top: 0;
        }
        
        /* CAT */
        .glitch {
            font-size: 1em;
            color: #ffffff;
            position: relative;
            animation: glitch 0.05s infinite, flicker 2s infinite;
        }

        .flicker-text1 {
            color: #c3c3c3;
            animation: flicker1 2s infinite;
        }

        @keyframes glitch {
            0%, 100% {
                transform: translate(0);
            }
            10% {
                transform: translate(-0.5px, 0.5px);
            }
            20% {
                transform: translate(-0.5px, -0.5px);
            }
            30% {
                transform: translate(0.5px, 0.5px);
            }
            40% {
                transform: translate(0.5px, -0.5px);
            }
            50% {
                transform: translate(-0.5px, 0.5px);
            }
            60% {
                transform: translate(-0.5px, -0.5px);
            }
            70% {
                transform: translate(0.5px, 0.5px);
            }
            80% {
                transform: translate(0.5px, -0.5px);
            }
            90% {
                transform: translate(-0.5px, 0.5px);
            }
        }

        @keyframes flicker {
            0%, 18%, 22%, 25%, 53%, 57%, 100% {
                opacity: 1;
            }
            20%, 24%, 55% {
                opacity: 0;
            }
        }

        .container {
            display: flex;
            width: 100%;
            height: 100%;
        }

        .center-half {
            width: 100%;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            padding: 20px;
            color: white;
            text-align: center;
            position: relative;
            z-index: 5;
            /* background-color: #000000; */
        }
    </style>
</head>
<body>
    <div class="nav-panel">
        <a href="login.php">Войти</a>
    </div>

    <div class="main-container">
        <h1>Киберпанк Панель</h1>
        <div class="bar-container">
            <div class="bar">
                <div class="bar-fill" style="width: <?php echo ($users_count / 100) * 100; ?>%;">
                    <span class="bar-label" id="user-label"><?php echo $users_count; ?> Пользователей</span>
                </div>
            </div>
            <div class="bar">
                <div class="bar-fill" style="width: <?php echo ($tasks_count / 200) * 100; ?>%;">
                    <span class="bar-label" id="task-label"><?php echo $tasks_count; ?> Задач</span>
                </div>
            </div>
        </div>

        <div class="info-section">
            <h2>Информация о платформе</h2>
            <p>Эта CTF платформа предназначена для проведения соревнований по компьютерной безопасности. Здесь вы можете найти задачи различной сложности и уровни. Вы также можете отслеживать прогресс своих решений и взаимодействовать с другими участниками.</p>
            <p>Пожалуйста, убедитесь, что при загрузке решений вы следуете всем рекомендациям и инструкциям, предоставленным для каждой задачи.</p>
        </div>
    </div>
    <div class="container">
        <!-- Left Section -->
        <div class="center-half">
            <div class="interactive-section" id="logo-center">
                <img src="images/tongue.png" alt="obey" style="display: none;">
            </div>
        </div>

    <script src="js/main.js"></script>
    <script>
        document.addEventListener("DOMContentLoaded", function() {

            new NextParticle({
                image: document.querySelector("#logo-center img"),
                color: "#FF0000",
                maxWidth: 500,
                maxHeight: 500,
                particleGap: 2,
                noise: 15,
                speed: 0.1,
            });
        });
    </script>


    </div>
</body>
</html>
