from alembic import op
import sqlalchemy as sa

# Revision identifiers, used by Alembic.
revision = '1234567890ab'
down_revision = None
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        'intraday_prices',
        sa.Column('id', sa.Integer, primary_key=True, autoincrement=True),
        sa.Column('timestamp', sa.DateTime, nullable=False),
        sa.Column('open', sa.Float, nullable=True),
        sa.Column('high', sa.Float, nullable=True),
        sa.Column('low', sa.Float, nullable=True),
        sa.Column('close', sa.Float, nullable=True),
        sa.Column('volume', sa.Float, nullable=True),
        sa.Column('stock_symbol', sa.String(50), nullable=False)
    )


def downgrade():
    op.drop_table('intraday_prices')
