<?php
require_once "config.php";
is_logged();

if (isset($_FILES['profile_image'])) {
    $errors = [];
    $file_name = $_FILES['profile_image']['name'];
    $file_size = $_FILES['profile_image']['size'];
    $file_tmp = $_FILES['profile_image']['tmp_name'];
    $file_type = $_FILES['profile_image']['type'];
    
    // Define allowed file types
    $allowed_types = ['image/jpeg', 'image/jpg', 'image/png', 'image/gif'];

    // Check file type
    if (!in_array($file_type, $allowed_types)) {
        $errors[] = "Invalid file type. Only JPEG, JPG, PNG, and GIF files are allowed.";
    }

    // Check file size (5MB limit)
    if ($file_size > 5242880) {
        $errors[] = "File size must be less than 5 MB.";
    }

    // If there are no errors, process the uploaded file
    if (empty($errors)) {
        $userId = $_POST['user_id']; // Assuming you pass the user ID
        $target_dir = "profiles/";
        $target_file = $target_dir . $userId . ".jpg"; // Save as user ID in JPEG format

        // Load the image based on its type
        switch ($file_type) {
            case 'image/jpeg':
            case 'image/jpg':
                $image = imagecreatefromjpeg($file_tmp);
                break;
            case 'image/png':
                $image = imagecreatefrompng($file_tmp);
                if ($image === false) {
                    $errors[] = "Failed to create image from PNG file.";
                }
                break;
            case 'image/gif':
                $image = imagecreatefromgif($file_tmp);
                if ($image === false) {
                    $errors[] = "Failed to create image from GIF file.";
                }
                break;
            default:
                echo json_encode(['success' => false, 'message' => 'Unsupported image type.']);
                exit;
        }

        // Check if the image was created successfully
        if (isset($image) && $image === false) {
            echo json_encode(['success' => false, 'message' => implode(", ", $errors)]);
            exit;
        }

        // Resize the image to 150x150
        $resized_image = resizeImage($image, 150, 150);

        // Save the resized image as JPEG
        if (imagejpeg($resized_image, $target_file, 100)) {
            echo json_encode(['success' => true, 'message' => 'Image uploaded and resized successfully.']);
        } else {
            echo json_encode(['success' => false, 'message' => 'Failed to save the image.']);
        }

        // Free up memory
        imagedestroy($image);
        imagedestroy($resized_image);
    } else {
        echo json_encode(['success' => false, 'message' => implode(", ", $errors)]);
    }
} else {
    echo json_encode(['success' => false, 'message' => 'No file uploaded.']);
}

// Function to resize the image
function resizeImage($image, $width, $height) {
    $original_width = imagesx($image);
    $original_height = imagesy($image);
    $image_p = imagecreatetruecolor($width, $height);

    // Resize the image
    imagecopyresampled($image_p, $image, 0, 0, 0, 0, $width, $height, $original_width, $original_height);

    return $image_p; // Return the resized image
}
?>
