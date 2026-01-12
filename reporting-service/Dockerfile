FROM python:3.10-slim

# Set working directory
WORKDIR /app

# Copy requirements file
COPY requirements.txt .

# Install dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy application source
COPY . .

# Expose port and command to run app
EXPOSE 5000
CMD ["flask", "run", "--host=0.0.0.0"]
