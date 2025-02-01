<?php
// upload_image.php

// Check if a file was uploaded
if (isset($_FILES['profile_image'])) {
    $errors = [];
    $file_name = $_FILES['profile_image']['name'];
    $file_size = $_FILES['profile_image']['size'];
    $file_tmp = $_FILES['profile_image']['tmp_name'];
    $file_type = $_FILES['profile_image']['type'];
    
    // Define allowed file types
    $allowed_types = ['image/jpeg', 'image/png', 'image/gif'];

    // Check file type
    if (!in_array($file_type, $allowed_types)) {
        $errors[] = "Invalid file type. Only JPG, PNG, and GIF files are allowed.";
    }

    // Check file size (5MB limit)
    if ($file_size > 5242880) {
        $errors[] = "File size must be less than 5 MB.";
    }

    // If there are no errors, move the uploaded file
    if (empty($errors)) {
        $userId = $_POST['user_id']; // Assuming you pass the user ID
        $target_dir = "profiles/";
        $target_file = $target_dir . $userId . ".jpg"; // Save as user ID

        // Move the uploaded file to the target directory
        if (move_uploaded_file($file_tmp, $target_file)) {
            echo json_encode(['success' => true, 'message' => 'Image uploaded successfully.']);
        } else {
            echo json_encode(['success' => false, 'message' => 'Failed to move uploaded file.']);
        }
    } else {
        echo json_encode(['success' => false, 'message' => implode(", ", $errors)]);
    }
} else {
    echo json_encode(['success' => false, 'message' => 'No file uploaded.']);
}
?>
