from django.http import HttpResponse
from django.shortcuts import render
from django.views.decorators.csrf import csrf_exempt
from django.utils.decorators import method_decorator
from django.views import View
from .models import StockData
import requests
import os
import yfinance as yf
from datetime import datetime, timedelta
from django.utils.timezone import make_aware


# API_KEY = os.environ.get('ALPHA_VANTAGE_API_KEY')
API_KEY = 'ABC'
DATA_SECTION = 'Time Series (1min)'
METADATA_SECTION = 'Meta Data'


@method_decorator(csrf_exempt, name='dispatch')
class StockDataView(View):

    def insert_data(self, data):
        stock_data = StockData(**data)
        stock_data.save()

    def get_stock_data_alphavantage(self, symbol):
        base_url = 'https://www.alphavantage.co/query'
        function = 'TIME_SERIES_INTRADAY'
        interval = '1min'

        params = {
            'function': function,
            'symbol': symbol,
            'interval': interval,
            'apikey': API_KEY,
            'adjusted': 'true',
        }

        response = requests.get(base_url, params=params)
        data = response.json()

        if DATA_SECTION in data:
            # get the timezone from the METADATA_SECTION
            timezone = data[METADATA_SECTION]['6. Time Zone']
            if not timezone:
                timezone = 'US/Eastern'
                
            # for each item on the data[DATA_SECTION].items() list, insert one record
            for timestamp, values in data[DATA_SECTION].items():
                data = {
                    'symbol': symbol,
                    'price_open': float(values['1. open']),
                    'price_high': float(values['2. high']),
                    'price_low': float(values['3. low']),
                    'price_close': float(values['4. close']),
                    'volume': float(values['5. volume']),
                    'timestamp': timestamp,
                    'timezone': timezone,
                }
                # day, hour, minute, day_of_week = get_date_information(timestamp)
                self.insert_data(data)
            return data
        else:
            return None

    def get_stock_data(self, symbol):
        # end_date = today
        end_date = datetime.utcnow().strftime('%Y-%m-%d')
        # start_date = today - 1 day
        start_date = (datetime.utcnow() - timedelta(days=1)).strftime('%Y-%m-%d')
        data = yf.download(symbol, interval='1m', start=start_date, end=end_date)
        for index, row in data.iterrows():
            values = row.to_dict()
            timezone = 'UTC' 
            timestamp = index.strftime('%Y-%m-%d %H:%M:%S')
            timestamp = make_aware(datetime.strptime(timestamp, '%Y-%m-%d %H:%M:%S'))

            record = {
                'symbol': symbol,
                'price_open': float(values['Open']),
                'price_high': float(values['High']),
                'price_low': float(values['Low']),
                'price_close': float(values['Close']),
                'volume': float(values['Volume']),
                'timestamp': timestamp,
                'timezone': timezone,
            }
            self.insert_data(record)

        return True


    def fetch_and_insert_data(self):
        # Read S&P 500 symbols from a file
        with open('./settings/sp500_symbols.txt', 'r') as file:
            sp500_symbols = [symbol.strip() for symbol in file]

        for symbol in sp500_symbols:
            stock_data = self.get_stock_data(symbol)
            if stock_data:
                print(f"Data inserted for {symbol}")

    def get(self, request, *args, **kwargs):
        self.fetch_and_insert_data()
        return HttpResponse("Data fetched and inserted successfully.")

# def get_date_information(date_string):
#     # Parse the date string
#     date_object = datetime.strptime(date_string, '%Y-%m-%d %H:%M:%S')

#     # Extract day, hour, minute, and day of the week
#     day = date_object.day
#     hour = date_object.hour
#     minute = date_object.minute
#     day_of_week = date_object.strftime('%A')  # Full day name

#     return day, hour, minute, day_of_week

# def weeks_between_dates(end_date_str):
#     start_date_str = datetime.utcfromtimestamp(0)
#     # Parse the date strings
#     start_date = datetime.strptime(start_date_str, '%Y-%m-%d')
#     end_date = datetime.strptime(end_date_str, '%Y-%m-%d')

#     # Calculate the difference between dates
#     date_difference = end_date - start_date

#     # Calculate the number of weeks
#     weeks = date_difference.days // 7

#     return weeks
