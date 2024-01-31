# stock_data_project/urls.py
from django.urls import path
from .views import StockDataView

urlpatterns = [
    path('', StockDataView.as_view(), name='fetch_and_insert_data'),
]
