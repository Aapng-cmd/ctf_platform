<?php
require_once "config.php";
$conn = start_conn();

is_logged();


$user_info = get_user_info($conn);

if ($user_info['group_type'] === 0)
{
	header("Location: home.php");
    exit;
}

$conn = start_conn();
session_start();

function create_task($conn, $name, $category, $description, $level, $cost, $hosting, $files, $flag, $solution, $readme)
{
    $category_id = get_category_id($conn, $category);
    $stmt = $conn->prepare("INSERT INTO tasks (name, category_id, description, level, author_id, cost, hosting, files, flag, solution, readme) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)");
    $stmt->bind_param("sissiisssss", $name, $category_id, $description, $level, $_SESSION['id'], $cost, $hosting, $files, $flag, $solution, $readme);
    
    if ($stmt->execute()) {
        $stmt->close();
        return true;
    }
    $stmt->close();
    return false;
}

function get_category_id($conn, $category)
{
    $stmt = $conn->prepare('SELECT id FROM categories WHERE name = ?');
    $stmt->bind_param("s", $category);
    $stmt->execute();
    $stmt->store_result();
    $stmt->bind_result($id);
    $stmt->fetch();
    $stmt->close();
    return $id;
}

function parse_readme($content) {
    $parsedData = [];
    
    // Split the content into lines
    $lines = explode("\n", $content);
    
    // Initialize variables to hold the current section
    $currentSection = null;

    // Loop through each line
    foreach ($lines as $line) {
        // Trim whitespace from the line
        $line = trim($line);
        
        // Check for headings
        if (preg_match('/^# (.+)$/', $line, $matches)) {
            // Main heading
            $currentSection = 'title';
            $parsedData['title'] = $matches[1];
        } elseif (preg_match('/^## (.+)$/', $line, $matches)) {
            // Subheading
            $currentSection = strtolower(str_replace(' ', '_', $matches[1])); // Convert to lowercase and replace spaces with underscores
            $parsedData[$currentSection] = ''; // Initialize the section
        } elseif ($currentSection) {
            // Add line to the current section
            if ($line !== '') {
                $parsedData[$currentSection] .= ($parsedData[$currentSection] ? "\n" : '') . $line;
            }
        }
    }

    return $parsedData;
}

function getDockerComposeUrl($dockerComposeFile, $hosting, $zip) {
    // Read the contents of the file
    $contents = $zip->getFromName($dockerComposeFile);
	
	$lines = explode("\n", $contents);

    // Initialize the port variable
    $port = null;

    // Flag to indicate if we are in a ports section
    $inPortsSection = false;

    // Loop through each line to find the ports
    foreach ($lines as $line) {
        // Trim whitespace from the line
        $line = trim($line);

        // Check if the line starts with "ports:"
        if (strpos($line, 'ports:') === 0) {
            $inPortsSection = true; // We are now in the ports section
            continue; // Move to the next line
        }

        // If we are in the ports section, look for port mappings
        if ($inPortsSection) {
            // Check if the line starts with a dash (indicating a port mapping)
            if (strpos($line, '-') === 0) {
                // Extract the port mapping (e.g., "8080:80" or "3306")
                $portMapping = trim(substr($line, 1)); // Remove the leading dash
                $portMapping = explode(':', $portMapping); // Split by colon

                // Get the host port (the first part)
                $port = $portMapping[0]; // This will be the host port
                break; // Exit the loop after finding the first port
            }
        }
    }
    
    if ($hosting === "Y" && $port !== null) {
        return "http://127.0.0.1:" . $port;
    }

    // Return null or an empty string if conditions are not met
    return null;
}

if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_FILES['zip_file'])) {
    try
    {
        $errors = [];
        $file = $_FILES['zip_file'];

        if ($file['error'] !== UPLOAD_ERR_OK) {
            $errors[] = "Error uploading file.";
        }

        // Check file type
        $fileType = pathinfo($file['name'], PATHINFO_EXTENSION);
        if ($fileType !== 'zip') {
            $errors[] = "Uploaded file is not a ZIP file.";
        }

        if ($file['size'] > 5242880 * 1024) { // 5GB limit
            $errors[] = "File size must be less than 5 GB.";
        }

        $zip = new ZipArchive;
        if ($zip->open($file['tmp_name']) === TRUE) {
            if ($zip->locateName('README.md') !== false) {
                $readmeContent = $zip->getFromName('README.md');
                $parsedData = parse_readme($readmeContent);
				#print_r($parsedData);
				$url = getDockerComposeUrl("docker-compose.yml", $parsedData['hosting'], $zip);
				
				if ($parsedData['files'] !== "N")
				{
					$files = "./uploads/" . $parsedData['title'];
					$zip->extractTo($files);
				}
				else { $files = null; }
				
                create_task($conn, $parsedData['title'], $parsedData['category'], $parsedData['description'], $parsedData['level'], $parsedData['cost'], $url, $files . "/" . $parsedData['files'], $parsedData['flag'], $parsedData['solution'], base64_encode($readmeContent));
                
                if ($zip->locateName('docker-compose.yml') !== false)
                {
                	$extractToPath = "./tasks/" . $parsedData['title'];
                	mkdir($extractToPath, 0755, true);
                	$zip->extractTo($extractToPath);
//                	chdir($extractToPath);
//                	
//		            $command = 'docker-compose up -d && docker ps';
//					exec($command, $output, $return_var);

//					if ($return_var === 0) {
//						echo "Docker Compose started successfully in $extractToPath.";
//					} else {
//						echo "Failed to start Docker Compose. Return code: $return_var.<br>";
//					}
                }
                
            } else {
                $errors[] = "Файл README.md не найден в загруженном ZIP-файле.";
            }
        } else {
            $errors[] = "Не удалось открыть ZIP-файл.";
        }
    }
    catch (ExceptionType $e)
    {
        $errors[] = "$e";
    }
}

$conn->close();
?>

<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Создать проект</title>
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

        .upload-container {
            max-width: 600px;
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

        p {
            color: #00ffcc;
            text-align: center;
            margin-bottom: 20px;
        }

        code {
            background-color: #2a2a2a;
            padding: 2px 5px;
            border-radius: 3px;
            color: #ff007f;
        }

        pre {
            background-color: #2a2a2a;
            padding: 10px;
            border-radius: 5px;
            color: #00ffcc;
            overflow-x: auto;
        }

        input[type="file"] {
            display: block;
            margin: 20px auto;
            padding: 10px;
            font-size: 16px;
            color: #00ffcc;
            background-color: #2a2a2a;
            border: 1px solid #00ffcc;
            border-radius: 5px;
            cursor: pointer;
        }

        button {
            background-color: #00ffcc;
            color: #000;
            border: none;
            border-radius: 5px;
            padding: 10px 20px;
            font-size: 16px;
            cursor: pointer;
            display: block;
            margin: 0 auto;
            transition: background 0.3s;
        }

        button:hover {
            background-color: #ff007f; /* Change color on hover */
        }
        .warning {
			background-color: #fff3cd; /* Light yellow background */
			color: #856404; /* Darker yellow/brown text color */
			border: 1px solid #ffeeba; /* Light yellow border */
			padding: 15px; /* Padding around the text */
			border-radius: 5px; /* Rounded corners */
			margin: 20px 0; /* Margin above and below the warning */
			font-weight: bold; /* Bold text */
			text-align: center; /* Center the text */
		}
    </style>
</head>
<body>
    <div class="nav-panel">
        <a href="home.php">Главная</a>
        <a href="logout.php">Выйти</a>
    </div>

    <div class="upload-container">
        <h1>Загрузить ZIP файл</h1>
        <p>Пожалуйста, убедитесь, что в ZIP-файле содержится файл <code>README.md</code> с полным проектом.</p>
        
        <h2>Пример README</h2>
        <pre>
# task_name

## Category
Forensics

## Description
well, yeah
hah

## Level
Easy

## Cost
150

## Files
N (file.ext)

## Hosting
Y/N

## Flag
flag_plug

## Solution
maybe, heh
&ltcode&gtThis could be&lt/code&gt
        </pre>
		<h2 class="warning">Убедитесь, что если нужны доп. файлы для решения, то они должны быть упакованы в архив, а сам архив упакован в основной. Если указан хостинг "Y", то в архиве должен быть docker-compose.yml</h2>
        <form action="" method="POST" enctype="multipart/form-data">
            <input type="file" name="zip_file" accept=".zip" required>
            <button type="submit">Загрузить</button>
        </form>
    </div>
</body>
</html>
