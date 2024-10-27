import yfinance as yf
import pandas as pd
import numpy as np
from flask import Flask, render_template_string

# Function to fetch tickers of major indices
def get_index_tickers():
    sp500 = pd.read_html('https://en.wikipedia.org/wiki/List_of_S%26P_500_companies')[0]['Symbol'].tolist()
    djia = pd.read_html('https://en.wikipedia.org/wiki/Dow_Jones_Industrial_Average')[1]['Symbol'].tolist()
    nasdaq = pd.read_html('https://en.wikipedia.org/wiki/NASDAQ-100')[3]['Ticker'].tolist()
    return list(set(sp500 + djia + nasdaq))

# Function to calculate EMAs and filter stocks
def screen_stocks(tickers):
    filtered_stocks = []
    for ticker in tickers:
        try:
            stock_data = yf.download(ticker, period='6mo', interval='1d')
            if stock_data.empty:
                continue

            stock_data['EMA20'] = stock_data['Close'].ewm(span=20, adjust=False).mean()
            stock_data['EMA25'] = stock_data['Close'].ewm(span=25, adjust=False).mean()
            stock_data['EMA30'] = stock_data['Close'].ewm(span=30, adjust=False).mean()
            stock_data['EMA35'] = stock_data['Close'].ewm(span=35, adjust=False).mean()
            stock_data['EMA40'] = stock_data['Close'].ewm(span=40, adjust=False).mean()

            latest_data = stock_data.iloc[-1]

            if (
                latest_data['EMA20'] > latest_data['EMA25'] > latest_data['EMA30'] > latest_data['EMA35'] > latest_data['EMA40']
                and latest_data['Close'] >= latest_data['EMA20']
            ):
                filtered_stocks.append(ticker)
        except Exception as e:
            print(f"Error processing {ticker}: {e}")

    return filtered_stocks

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
