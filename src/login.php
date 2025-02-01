<?php
require_once "config.php";
session_start();
$conn = start_conn();

$error = "";

function login($conn, $username, $password)
{
	$stmt = $conn->prepare("SELECT id, password FROM users WHERE username = ?");
    $stmt->bind_param("s", $username);
    $stmt->execute();
    $stmt->store_result();
    if ($stmt->num_rows > 0) {
        $stmt->bind_result($id, $hashedPassword);
        $stmt->fetch();
        if (password_verify($password, $hashedPassword)) {
            $_SESSION['id'] = $id;
            $stmt->close();
            return true;
        } else {
        	$stmt->close();
            return false;
        }
    } else {
    	$stmt->close();
        return false;
    }
}

function register($conn, $username, $password, $user_type) {
    $stmt = $conn->prepare("SELECT id FROM users WHERE username = ?");
    $stmt->bind_param("s", $username);
    $stmt->execute();
    $stmt->store_result();
    if ($stmt->num_rows > 0) {
    	$stmt->close();
        return false;
    } else {
        $hashedPassword = password_hash($password, PASSWORD_DEFAULT);
        $stmt = $conn->prepare("INSERT INTO users (username, password, group_type) VALUES (?, ?, ?)");
        $stmt->bind_param("sss", $username, $hashedPassword, $user_type);
        
        if ($stmt->execute()) {
        	$stmt->close();
        	$stmt = $conn->prepare("SELECT id FROM users WHERE username = ?");
			$stmt->bind_param("s", $username);
			$stmt->execute();
			$stmt->store_result();
			$stmt->bind_result($id);
        	$stmt->fetch();
        	copy('profiles/default.jpg', "profiles/$id.jpg");
        	$stmt->close();
            return true;
        } else {
        	$stmt->close();
            return false;
        }
    }
}

if (isset($_POST['register']))
{
	$username = isset($_POST['username']) ? $_POST['username'] : null;
	$password = isset($_POST['password']) ? $_POST['password'] : null;
	$group_type = isset($_POST['group_type']) ? $_POST['group_type'] : null;
	#echo "$username|$password|$group_type";
	if ($username === null or $password === null or $group_type === null)
	{
		$error = "Не введено или имя, или пароль, или группа пользователя";
	}
	else
	{
		if (!register($conn, $username, $password, $group_type))
		{
			$error = "Что-то пошло не так";
		}
		else
		{
			$error = "Отлично, теперь можно войти";
		}
	}
}
else if (isset($_POST['login']))
{
	$username = isset($_POST['username']) ? $_POST['username'] : null;
	$password = isset($_POST['password']) ? $_POST['password'] : null;
	
	if ($username === null or $password === null)
	{
		$error = "Не введено или имя, или пароль";
	}
	else
	{
		if (!login($conn, $username, $password))
		{
			$error = "Неправильные логин/пароль";
		}
		else
		{
			header("Location: index.php");
		}
	}
}

$conn->close();
?>

<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Login/Register Page</title>
    <style>
        /* Reset some default browser styles */
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: Arial, sans-serif;
            background-color: #f0f2f5; /* Light gray background */
            display: flex;
            align-items: center;
            justify-content: center;
            height: 100vh; /* Full height of the viewport */
        }

        /* Container styling */
        .container {
            background-color: #fff; /* White background for the form */
            padding: 20px;
            border-radius: 8px; /* Rounded corners */
            box-shadow: 0 0 10px rgba(0, 0, 0, 0.1); /* Subtle shadow */
            width: 90%;
            max-width: 400px; /* Maximum width for larger screens */
        }

        /* Form styling */
        .login-form, .register-form {
            display: none; /* Initially hide both forms */
            flex-direction: column; /* Stack elements vertically */
        }

        /* Show login form by default */
        .login-form.active, .register-form.active {
            display: flex;
        }

        /* Form group styling */
        .form-group {
            margin-bottom: 15px; /* Space between form groups */
        }

        label {
            margin-bottom: 5px; /* Space between label and input */
            font-weight: bold;
        }

        /* Input styling */
        input[type="text"],
        input[type="password"],
        select {
            padding: 10px;
            border: 1px solid #ccc; /* Light gray border */
            border-radius: 4px; /* Slightly rounded corners */
            font-size: 16px; /* Font size for inputs */
        }
        
        p {
            font-size: 20px;
            font-style: italic;
            color: #333;
            letter-spacing: 2px;
            text-decoration: underline;
            text-decoration-color: #007bff; /* Change underline color */
            text-decoration-thickness: 3px; /* Thickness of the underline */
            margin: 20px 0;
        }

        /* Main button styling */
        button {
            padding: 10px;
            background-color: #007bff; /* Bootstrap primary color */
            color: #fff; /* White text */
            border: none; /* No border */
            border-radius: 4px; /* Slightly rounded corners */
            font-size: 16px; /* Font size for button */
            cursor: pointer; /* Pointer on hover */
            margin-top: 10px; /* Space above the button */
        }

        button:hover {
            background-color: #0056b3; /* Darker shade on hover */
        }

        /* Switch button styling */
        .switch-button {
            padding: 5px; /* Smaller padding for switch buttons */
            background-color: #28a745; /* Green background color */
            color: #fff; /* White text */
            border: none; /* No border */
            border-radius: 4px; /* Slightly rounded corners */
            font-size: 14px; /* Smaller font size */
            cursor: pointer; /* Pointer on hover */
            margin-top: 5px; /* Space above the button */
        }

        .switch-button:hover {
            background-color: #218838; /* Darker green on hover */
        }

        /* Responsive styling */
        @media (max-width: 480px) {
            body {
                padding: 10px; /* Padding for small screens */
            }

            .container {
                width: 100%; /* Full width on small screens */
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <form method="post" class="login-form active">
            <h2>Вход</h2>
            <div class="form-group">
                <label for="username">Имя</label>
                <input type="text" id="username" name="username" required>
            </div>
            <div class="form-group">
                <label for="password">Пароль</label>
                <input type="password" id="password" name="password" required>
            </div>
            <button type="submit" name="login">Вход</button>
            <button type="button" class="switch-button" id="switchToRegister">Нет аккаунта? Зарегистрироваться</button>
        </form>

        <form method="post" class="register-form">
            <h2>Регистрация</h2>
            <div class="form-group">
                <label for="username">Имя</label>
                <input type="text" id="username" name="username" required>
            </div>
            <div class="form-group">
                <label for="password">Пароль</label>
                <input type="password" id="password" name="password" required>
            </div>
            <div class="form-group">
                <label for="group_type">Кто ты?</label>
                <select id="group_type" name="group_type" required>
                    <option value="0">Пользователь</option>
                    <option value="1">Создатель</option>
                </select>
            </div>
            <button type="submit" name="register">Регистрация</button>
            <button type="button" class="switch-button" id="switchToLogin">Уже смешарик? Войти</button>
        </form>
        
        <p><?php echo isset($error) ? htmlspecialchars($error) : ''; ?></p>
    </div>

    <script>
        const loginForm = document.querySelector('.login-form');
        const registerForm = document.querySelector('.register-form');
        const switchToRegisterBtn = document.getElementById('switchToRegister');
        const switchToLoginBtn = document.getElementById('switchToLogin');

        // Show the register form and hide the login form
        switchToRegisterBtn.addEventListener('click', () => {
            loginForm.classList.remove('active');
            registerForm.classList.add('active');
        });

        // Show the login form and hide the register form
        switchToLoginBtn.addEventListener('click', () => {
            registerForm.classList.remove('active');
            loginForm.classList.add('active');
        });
    </script>
</body>
</html>
