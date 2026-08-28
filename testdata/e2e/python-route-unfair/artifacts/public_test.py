from route import select_route


def test_non_tie_routes():
    assert select_route(False, False) == 0
    assert select_route(False, True) == 2
    assert select_route(True, False) == 1
