def select_route(primary_healthy, replica_healthy):
    if primary_healthy:
        return 1
    return 2
