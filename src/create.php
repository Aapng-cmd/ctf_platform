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


function create_task($conn, $name, $category, $description, $level, $cost, $hosting, $files, $flag, $solution, $readme)
{
    $category_id = get_category_id($conn, $category);
    if ($category_id === null)
    {
        return false;
    }
    $stmt = $conn->prepare("INSERT INTO tasks (name, category_id, description, level, author_id, cost, hosting, files, flag, solution, readme) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)");
    
    // Sanitize inputs and assign to variables
    $sanitized_name = htmlspecialchars($name);
    $sanitized_description = htmlspecialchars($description);
    $sanitized_level = htmlspecialchars($level);
    $author_id = (int)$_SESSION['id'];
    $category_id = (int)$category_id;
    $sanitized_cost = (int)$cost;
    $sanitized_hosting = htmlspecialchars($hosting);
    $sanitized_files = htmlspecialchars($files);
    $sanitized_flag = htmlspecialchars($flag);
    $sanitized_solution = htmlspecialchars($solution);
    $sanitized_readme = htmlspecialchars($readme);

    // Bind parameters using the sanitized variables
    $stmt->bind_param("sissiisssss", $sanitized_name, $category_id, $sanitized_description, $sanitized_level, $author_id, $sanitized_cost, $sanitized_hosting, $sanitized_files, $sanitized_flag, $sanitized_solution, $sanitized_readme);
    
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

function getDockerComposeUrl($dockerComposeFile, $hosting, $zip, $newPort) {
    // Read the contents of the file
    $contents = $zip->getFromName($dockerComposeFile);
    
    $lines = explode("\n", $contents);

    // Initialize the port variable
    $port = null;

    // Flag to indicate if we are in a ports section
    $inPortsSection = false;

    // Loop through each line to find the ports
    foreach ($lines as $index => $line) {
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
                $portMappingParts = explode(':', $portMapping); // Split by colon

                // Replace the host port (the first part) with the new port
                $portMappingParts[0] = $newPort; // Set the new port

                // Reconstruct the port mapping line
                $lines[$index] = '      - "' . implode(':', $portMappingParts);
                break; // Exit the loop after modifying the first port
            }
        }
    }

    // Write the modified contents back to the zip file
    $newContents = implode("\n", $lines);
    $zip->addFromString($dockerComposeFile, $newContents);

    if ($hosting === "Y" && isset($portMappingParts[0])) {
        return "http://127.0.0.1:" . $portMappingParts[0];
    }

    // Return null or an empty string if conditions are not met
    return null;
}

function findPort($startPort = 1000, $maxPort = 65535) {
    // Get the list of used ports using netstat
    $usedPorts = [];
    $output = [];
    exec("netstat -tuln", $output); // Execute netstat command

    // Parse the output to find used ports
    foreach ($output as $line) {
        // Match lines that contain the port information
        if (preg_match('/:(\d+)\s/', $line, $matches)) {
            $usedPorts[] = (int)$matches[1]; // Store the port number
        }
    }
    
    // Fetch forbidden ports from the MySQL database

    $forbiddenPorts = [];
    $conn = start_conn();
    if ($conn) {
        $stmt = $conn->prepare("SELECT hosting FROM tasks");
        $stmt->execute();
        $stmt->store_result();
        $stmt->bind_result($hosting);

        while ($stmt->fetch()) {
            if (preg_match('/:(\d+)/', $hosting, $matches)) {
                $forbiddenPorts[] = (int)$matches[1]; // Store the port number
            }
        }
        $stmt->close(); // Close the statement
    }
    $conn->close(); // Close the connection

    // Combine used ports from netstat and forbidden ports from the database
    $allUsedPorts = array_merge($usedPorts, $forbiddenPorts);

    // Search for an available port
    for ($port = $startPort; $port <= $maxPort; $port++) {
        if (!in_array($port, $usedPorts) && !in_array($port, $forbiddenPorts)) {
            return $port; // Return the first available port
        }
    }

    return null; // No available port found
}



function create_hint($conn, $hint_data, $taskname)
{
    $stmt = $conn->prepare("INSERT INTO hints (task_id, description, cost) VALUES ((SELECT id FROM tasks WHERE name = ?), ?, ?)");
    $description = htmlspecialchars($hint_data[0]);
    $cost = (int)htmlspecialchars($hint_data[1]);
    $stmt->bind_param("ssi", $taskname, $description, $cost);
    
    if ($stmt->execute()) {
        $stmt->close();
        return true;
    }
    $stmt->close();
    return false;
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
                $url = getDockerComposeUrl("docker-compose.yml", $parsedData['hosting'], $zip, findPort());
                
                if ($parsedData['files'] != "N")
                {
                    $files = "";
                    $files_prefix = "./uploads/" . $parsedData['title'];
                    $inside_files = explode(";", $parsedData['files']);
                    foreach ($inside_files as $inside_file) {
                        $zip->extractTo($files_prefix . "/", $inside_file);
                        $files .= $files_prefix . "/" . $inside_file . ";";
                    }
                }
                else { $files = null; }
                
                if (!create_task($conn, $parsedData['title'], $parsedData['category'], $parsedData['description'], $parsedData['level'], $parsedData['cost'], $url, $files, $parsedData['flag'], $parsedData['solution'], base64_encode($readmeContent)))
                {
                    $errors[] = "Проверь все файлы и правильность создания README";
                }
                
                if (isset($parsedData['hints']))
                {
                    
                    foreach (explode("\n", $parsedData['hints']) as $hint)
                    {
                        $hint_data = explode("|", $hint);
                        create_hint($conn, $hint_data, $parsedData['title']);
                    }
                }
                
                if ($zip->locateName('docker-compose.yml') !== false)
                {
                    $extractToPath = "/var/www/ctf_tasks/" . $parsedData['title'];
                    mkdir($extractToPath, 0755, true);
                    $zip->extractTo($extractToPath);
//                    chdir($extractToPath);
//                    
//                    $command = 'docker-compose up -d && docker ps';
//                    exec($command, $output, $return_var);

//                    if ($return_var === 0) {
//                        echo "Docker Compose started successfully in $extractToPath.";
//                    } else {
//                        echo "Failed to start Docker Compose. Return code: $return_var.<br>";
//                    }
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
N (file1.ext;file2.ext)

## Hosting
Y/N

## Flag
flag_plug

## Hints
hint1|10
hint2|20

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
    <p><?php print_r($errors); ?></p>
</body>
</html>
