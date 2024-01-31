from django.http import HttpResponse
from django.shortcuts import render
from django.views.decorators.csrf import csrf_exempt
from django.utils.decorators import method_decorator
from django.views import View
from .models import StockData
import requests
import os

# API_KEY = os.environ.get('ALPHA_VANTAGE_API_KEY')
DATA_SECTION = 'Time Series (1min)'


@method_decorator(csrf_exempt, name='dispatch')
class StockDataView(View):

    def insert_data(self, data):
        stock_data = StockData(**data)
        stock_data.save()

    def get_stock_data(self, symbol):
        base_url = 'https://www.alphavantage.co/query'
        function = 'TIME_SERIES_INTRADAY'
        interval = '1min'

        params = {
            'function': function,
            'symbol': symbol,
            'interval': interval,
            'apikey': API_KEY,
        }

        response = requests.get(base_url, params=params)
        data = response.json()

        if DATA_SECTION in data:
            latest_data = list(data[DATA_SECTION].items())[0][1]
            return {
                'symbol': symbol,
                'price_open': float(latest_data['1. open']),
                'price_high': float(latest_data['2. high']),
                'price_low': float(latest_data['3. low']),
                'price_close': float(latest_data['4. close']),
                'volume': int(latest_data['5. volume']),
            }
        else:
            return None

    def fetch_and_insert_data(self):
        # Read S&P 500 symbols from a file
        with open('./settings/sp500_symbols.txt', 'r') as file:
            sp500_symbols = [symbol.strip() for symbol in file]

        for symbol in sp500_symbols:
            stock_data = self.get_stock_data(symbol)
            if stock_data:
                self.insert_data(stock_data)
                print(f"Data inserted for {symbol}")

    def get(self, request, *args, **kwargs):
        self.fetch_and_insert_data()
        return HttpResponse("Data fetched and inserted successfully.")
