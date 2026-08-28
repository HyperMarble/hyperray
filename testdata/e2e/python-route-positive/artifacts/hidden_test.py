from route import select_route


def test_replica_routes():
    assert select_route(False, False) == 0
    assert select_route(False, True) == 2
