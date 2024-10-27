import yfinance as yf
import pandas as pd
import numpy as np
from flask import Flask, render_template_string
import requests
from bs4 import BeautifulSoup


# Function to fetch tickers of major indices
def get_index_tickers():
    # Get S&P 500 tickers
    sp500 = pd.read_html('https://en.wikipedia.org/wiki/List_of_S%26P_500_companies')[0]['Symbol'].tolist()

    # Get Nasdaq tickers
    nasdaq = get_tickers("https://en.wikipedia.org/wiki/Nasdaq-100", "#constituents > tbody > tr > td:nth-child(2)")

    # Get DJIA tickers
    djia =  get_tickers('https://en.wikipedia.org/wiki/Dow_Jones_Industrial_Average', "#constituents > tbody > tr > td:nth-child(3)")
    
    # remove duplicated items and return the list
    return list(set(sp500 + nasdaq + djia))


def get_tickers(url, selector):
  response = requests.get(url)
  response.raise_for_status()  # Check for request errors

  # Parse the page source with BeautifulSoup
  soup = BeautifulSoup(response.text, 'html.parser')

  # Locate all rows in the table
  rows = soup.select(selector)

  # Extract text from each row
  symbols = [row.get_text(strip=True) for row in rows]

  return symbols

def get_djia_tickers():
    djia = pd.read_html('https://en.wikipedia.org/wiki/Dow_Jones_Industrial_Average')[1]['Symbol']
    return djia

# Function to calculate EMAs and filter stocks
def screen_stocks(tickers):
    result = []
    for ticker in tickers:
        try:
            stock_data = yf.download(ticker, period='6mo', interval='1d')
            if stock_data.empty:
                continue

            stock_data = stock_data.reset_index()  # Ensure index is integer-based
            stock_data['EMA20'] = stock_data['Close'].ewm(span=20, adjust=False).mean()
            stock_data['EMA25'] = stock_data['Close'].ewm(span=25, adjust=False).mean()
            stock_data['EMA30'] = stock_data['Close'].ewm(span=30, adjust=False).mean()
            stock_data['EMA35'] = stock_data['Close'].ewm(span=35, adjust=False).mean()
            stock_data['EMA40'] = stock_data['Close'].ewm(span=40, adjust=False).mean()

            latest_data = stock_data.iloc[-1]
            ema20 = float(latest_data['EMA20'])
            ema25 = float(latest_data['EMA25'])
            ema30 = float(latest_data['EMA30'])
            ema35 = float(latest_data['EMA35'])
            ema40 = float(latest_data['EMA40'])
            close = float(latest_data['Close'])
            if (
                ema20 > ema25 and
                ema25 > ema30 and
                ema30 > ema35 and
                ema35 > ema40 and
                close >= ema20
            ):
                print(f"{ticker} passed the screening test")
                result.append(f"{ticker}")
        except Exception as e:
            print(f"Error processing {ticker}: {e}")

    return result

# Flask app to render HTML page
app = Flask(__name__)

@app.route('/')
def home():
    tickers = get_index_tickers()
    filtered_stocks = screen_stocks(tickers)
    html_template = """
    <!doctype html>
    <html lang="en">
      <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <title>Stock Screening Results</title>
      </head>
      <body>
        <div class="container">
          <h1 class="mt-5">Stock Screening Results</h1>
          <ul>
            {% for stock in stocks %}
              <li>{{ stock }}</li>
            {% endfor %}
          </ul>
        </div>
      </body>
    </html>
    """
    return render_template_string(html_template, stocks=filtered_stocks)

if __name__ == '__main__':
    app.run(debug=True)
