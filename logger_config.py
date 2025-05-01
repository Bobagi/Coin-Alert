import logging
from logging.handlers import TimedRotatingFileHandler
import os
import sys

class ColorFormatter(logging.Formatter):
    COLOR_CODES = {
        'DEBUG': '\033[37m',    # White
        'INFO': '\033[0m',      # Reset (default)
        'WARNING': '\033[33m',  # Yellow
        'ERROR': '\033[31m',    # Red
        'CRITICAL': '\033[41m', # Red background
    }
    RESET_CODE = '\033[0m'

    def format(self, record):
        level_color = self.COLOR_CODES.get(record.levelname, self.RESET_CODE)
        message = super().format(record)
        return f"{level_color}{message}{self.RESET_CODE}"

def setup_logger(name='app', user_id=None):
    logger = logging.getLogger(name)
    logger.setLevel(os.getenv('LOG_LEVEL', 'INFO'))

    log_dir = os.getenv("LOG_DIR", "logs")
    os.makedirs(log_dir, exist_ok=True)

    log_path = os.path.join(log_dir, f"{name}.log")

    formatter = logging.Formatter(
        fmt='[{asctime}] [{levelname:^8}] [{name}] {message}',
        datefmt='%Y-%m-%d %H:%M:%S',
        style='{'
    )

    color_formatter = ColorFormatter(
        fmt='[{asctime}] [{levelname:^8}] [{name}] {message}',
        datefmt='%Y-%m-%d %H:%M:%S',
        style='{'
    )

    if not logger.handlers:
        stream_handler = logging.StreamHandler(sys.stdout)
        stream_handler.setFormatter(color_formatter)
        logger.addHandler(stream_handler)

        file_handler = TimedRotatingFileHandler(log_path, when='D', backupCount=7, encoding='utf-8')
        file_handler.setFormatter(formatter)
        logger.addHandler(file_handler)

    if user_id:
        logger = logging.LoggerAdapter(logger, {'user_id': user_id})

    return logger
