#include <memory>
extern "C" int nondet_int();
int main(){ int c=nondet_int(); std::unique_ptr<int> p;
  if(c>0) p = std::unique_ptr<int>(new int(7));
  return *p; }    // null deref when c<=0
