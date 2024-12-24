import yfinance as yf
import time
import os
import json
import queue
from datetime import datetime
from sqlalchemy import create_engine, text
from sqlalchemy.orm import sessionmaker
# from alembic import command
# from alembic.config import Config
from concurrent.futures import ThreadPoolExecutor
import click


# Database connection setup
def get_db_connection():
    db_user = os.getenv("DB_USER")
    db_password = os.getenv("DB_PASSWORD")
    db_host = os.getenv("DB_HOST")
    db_port = os.getenv("DB_PORT", "5432")
    db_name = os.getenv("DB_NAME")

    if not all([db_user, db_password, db_host, db_name]):
        raise ValueError("Database credentials are not fully set in environment variables.")

    db_url = f"postgresql://{db_user}:{db_password}@{db_host}:{db_port}/{db_name}"
    engine = create_engine(db_url)
    Session = sessionmaker(bind=engine)
    return engine, Session


# # Apply migrations using Alembic
# def apply_migrations():
#     alembic_cfg = Config("alembic.ini")
#     command.upgrade(alembic_cfg, "head")


# Function to fetch and store intraday stock data
def fetch_and_store_intraday_data(stock_symbol, interval="1m"):
    try:
        # Fetch intraday data using yfinance
        data = yf.download(tickers=stock_symbol, interval=interval, period="1d")

        if data.empty:
            print(f"No data fetched for the stock {stock_symbol}.")
            return

        # Add a timestamp column
        data.reset_index(inplace=True)
        data['Timestamp'] = datetime.utcnow()

        # Connect to the database
        engine, Session = get_db_connection()
        session = Session()

        # Insert data into the database
        for _, row in data.iterrows():
            session.execute(
                text("""
                INSERT INTO intraday_prices (timestamp, open, high, low, close, volume, stock_symbol)
                VALUES (:timestamp, :open, :high, :low, :close, :volume, :stock_symbol)
                """),
                {
                    "timestamp": row["Datetime"],
                    "open": row["Open"],
                    "high": row["High"],
                    "low": row["Low"],
                    "close": row["Close"],
                    "volume": row["Volume"],
                    "stock_symbol": stock_symbol
                }
            )
        
        session.commit()
        session.close()

    except Exception as e:
        print(f"An error occurred for {stock_symbol}: {e}")


# Worker function to process stock symbols
def worker(stock_queue):
    while True:
        stock_symbol = stock_queue.get()
        if stock_symbol is None:  # Exit signal
            break
        fetch_and_store_intraday_data(stock_symbol)
        stock_queue.put(stock_symbol)  # Re-enqueue the stock symbol
        time.sleep(60)  # Wait before processing again


# Command-line interface using Click
@click.group()
def cli():
    pass


@cli.command()
@click.option('--stock-file', type=click.Path(exists=True), required=True, help="Path to the JSON file containing stock symbols.")
@click.option('--workers', default=1, help="Number of worker threads.")
def start(stock_file, workers):
    "Start the stock data processing server."
    # apply_migrations()

    # Load stock symbols from the JSON file
    with open(stock_file, 'r') as f:
        stocks = json.load(f).get("stocks", [])

    if not stocks:
        print("No stocks found in the file.")
        return

    # Initialize the queue
    stock_queue = queue.Queue()
    for stock in stocks:
        stock_queue.put(stock)

    # Start worker threads
    with ThreadPoolExecutor(max_workers=workers) as executor:
        for _ in range(workers):
            executor.submit(worker, stock_queue)


if __name__ == "__main__":
    cli()
