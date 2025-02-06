# Use the official Ubuntu image
FROM ubuntu:20.04

# Set environment variables to avoid interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Update package list and install necessary packages
RUN apt-get update && \
    apt-get install -y \
    apache2 \
    php \
    libapache2-mod-php \
    php-mysqli \
    php-zip \
    php-gd \
    zip \
    nano \
    docker.io \
    systemctl \
    sudo \
    journalctl \
    apache2ctl \
    a2ensite \
    docker-compose && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Enable Apache mod_rewrite
RUN a2enmod rewrite

# Set the working directory
WORKDIR /var/www/html

# Copy the source code into the container
COPY src/ .

# Set permissions (optional, adjust as needed)
RUN chown -R www-data:www-data /var/www/html

# Expose port 80
EXPOSE 80

# Create a start script
RUN mkdir /tasks
RUN echo "#!/bin/bash" > /tasks/start.sh
RUN echo "a2ensite /etc/apache2/sites-available/mydomain.conf" >> /tasks/start.sh
RUN echo "service apache2 start" >> /tasks/start.sh
RUN echo "dockerd &" >> /tasks/start.sh
RUN echo "sleep 10" >> /tasks/start.sh
RUN echo "tail -f /var/log/apache2/error.log" >> /tasks/start.sh  # Keep the container running
RUN chmod +x /tasks/start.sh
RUN chown -R www-data:www-data /tasks

# Start the script
CMD ["/tasks/start.sh"]
