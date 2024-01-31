from django.db import models

class StockData(models.Model):
    symbol = models.CharField(max_length=10)
    price_open = models.DecimalField(max_digits=26, decimal_places=16)
    price_high = models.DecimalField(max_digits=26, decimal_places=16)
    price_low = models.DecimalField(max_digits=26, decimal_places=16)
    price_close = models.DecimalField(max_digits=26, decimal_places=16)
    volume = models.DecimalField(max_digits=14, decimal_places=4)
    timestamp = models.DateTimeField(null=False)
    timezone = models.CharField(max_length=20, null=True)
    m1 = models.BooleanField(default=False)
    m3 = models.BooleanField(default=False)
    m5 = models.BooleanField(default=False)
    m15 = models.BooleanField(default=False)
    m30 = models.BooleanField(default=False)
    m45 = models.BooleanField(default=False)
    h1 = models.BooleanField(default=False)
    h2 = models.BooleanField(default=False)
    h4 = models.BooleanField(default=False)
    h8 = models.BooleanField(default=False)
    h12 = models.BooleanField(default=False)
    h16 = models.BooleanField(default=False)
    d1 = models.BooleanField(default=False)
    d2 = models.BooleanField(default=False)
    d3 = models.BooleanField(default=False)
    d4 = models.BooleanField(default=False)
    d5 = models.BooleanField(default=False)
    d6 = models.BooleanField(default=False)
    w1 = models.BooleanField(default=False)
    w2 = models.BooleanField(default=False)
    w3 = models.BooleanField(default=False)
    w4 = models.BooleanField(default=False)

    # create an index for all models.BooleanField fields
    class Meta:
        indexes = [
            models.Index(fields=['symbol', 'm1', 'm3', 'm5', 'm15', 'm30', 'm45', 'h1', 'h2', 'h4', 'h8', 'h12', 'h16', 'd1', 'd2', 'd3', 'd4', 'd5', 'd6', 'w1', 'w2', 'w3', 'w4']),
            models.Index(fields=['symbol', 'timestamp']),
            models.Index(fields=['symbol']),
        ]
        
    def __str__(self):
        return f"{self.symbol} - {self.timestamp}"
