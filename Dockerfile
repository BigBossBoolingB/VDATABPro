# Use an official Python runtime as a parent image
FROM python:3.11-slim

# Set the working directory in the container
WORKDIR /app

# Copy the requirements file into the container at /app
COPY requirements.txt .

# Install any needed packages specified in requirements.txt
# --no-cache-dir: Disables the cache which is not needed for final images
# --trusted-host: Sometimes needed in firewalled environments
RUN pip install --no-cache-dir --trusted-host pypi.python.org -r requirements.txt

# Copy the rest of the application's source code from the host to the container at /app
COPY . .

# Define the entry point for the container.
# This makes the container executable, running the CLI by default.
# Example usage: docker run chronos-app propose --data "1,2,3,4"
ENTRYPOINT ["python", "cli.py"]

# Set a default command (can be overridden). Here, we show the help message.
CMD ["--help"]
