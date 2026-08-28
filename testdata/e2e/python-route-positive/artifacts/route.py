def select_route(primary_healthy, replica_healthy):
    if primary_healthy:
        return 1
    if replica_healthy:
        return 2
    return 0
