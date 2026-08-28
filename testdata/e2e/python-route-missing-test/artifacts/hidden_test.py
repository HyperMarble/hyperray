from route import select_route


def test_replica_route():
    assert select_route(False, True) == 2
