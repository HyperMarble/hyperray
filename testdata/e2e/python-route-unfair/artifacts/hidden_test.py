from route import select_route


def test_both_routes():
    assert select_route(True, True) == 1
