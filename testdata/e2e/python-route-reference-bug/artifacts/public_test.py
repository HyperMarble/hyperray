from route import select_route


def test_primary_routes():
    assert select_route(True, False) == 1
    assert select_route(True, True) == 1
