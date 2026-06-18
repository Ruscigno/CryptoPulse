from stock_screener.rule import classify, qualifies

def test_classify():
    peaks = [(1, 70.0), (5, 60.0)]    # min peak = 60
    valleys = [(2, 30.0), (6, 40.0)]  # max valley = 40
    assert classify(65, peaks, valleys) == "high"
    assert classify(35, peaks, valleys) == "low"
    assert classify(50, peaks, valleys) == "neutral"

def test_classify_precedence_high():
    # overlapping (min peak 40 <= max valley 60): 50 satisfies both -> high wins
    assert classify(50, [(1, 40.0)], [(2, 60.0)]) == "high"

def test_classify_empty():
    assert classify(50, [], []) == "neutral"

def test_qualifies():
    assert qualifies(1, 3, "any")
    assert not qualifies(0, 3, "any")
    assert qualifies(3, 3, "all")
    assert not qualifies(2, 3, "all")
    assert not qualifies(0, 0, "all")
    assert qualifies(2, 3, "min:2")
    assert not qualifies(1, 3, "min:2")
    assert not qualifies(5, 3, "min:0")
    assert not qualifies(5, 3, "min:abc")
    assert not qualifies(5, 3, "bogus")
