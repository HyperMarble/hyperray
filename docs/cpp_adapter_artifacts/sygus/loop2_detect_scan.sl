; C++ loop #2 — detect_scan(n) from replay_reader.cpp:34
;   size_t i = 0, seen = 0;
;   while (i < n) { i = i + 1; seen = seen + 2; }
; Two coupled counters. The interesting answer is the EQUALITY seen = 2i,
; which is what ESBMC's k-induction needs and cannot find alone (measured).
(set-logic LIA)

(synth-inv inv_fun ((i Int) (seen Int) (n Int)))

(define-fun pre_fun ((i Int) (seen Int) (n Int)) Bool
  (and (= i 0) (= seen 0) (>= n 0)))

(define-fun trans_fun ((i Int) (seen Int) (n Int)
                       (i! Int) (seen! Int) (n! Int)) Bool
  (and (= n! n)
       (< i n)
       (= i! (+ i 1))
       (= seen! (+ seen 2))))

(define-fun post_fun ((i Int) (seen Int) (n Int)) Bool
  (=> (>= i n) (= seen (* 2 n))))

(inv-constraint inv_fun pre_fun trans_fun post_fun)

(check-synth)
