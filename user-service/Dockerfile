FROM php:8.1-apache

# Instalasi ekstensi yang diperlukan dan git
RUN apt-get update && apt-get install -y \
    libzip-dev \
    zip \
    unzip \
    git \
    && docker-php-ext-install mysqli zip

# Instal Composer
COPY --from=composer:latest /usr/bin/composer /usr/bin/composer

# Set working directory
WORKDIR /var/www/html

# Copy composer.json dan install dependensi
COPY composer.json .
RUN composer install --no-dev --optimize-autoloader

# Copy source code aplikasi
COPY src/ .

# Atur rewrite rule untuk routing
RUN a2enmod rewrite
COPY 000-default.conf /etc/apache2/sites-available/000-default.conf