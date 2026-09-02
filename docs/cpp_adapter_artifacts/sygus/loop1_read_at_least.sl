; C++ loop #1 — read_at_least(want, cap) from replay_reader.cpp:21
;   size_t len = 0;
;   while (len < want && len < cap) {
;       size_t got = 8192;
;       if (len + got > cap) got = cap - len;
;       len = len + got;
;   }
; Question: is len bounded by cap?  Constants 8192 and 65536 are the two
; extracted by stage 1 (globals SCRATCH_STEP / MAX_DETECTION_PREFIX_LEN).
(set-logic LIA)

(synth-inv inv_fun ((len Int) (want Int) (cap Int)))

(define-fun pre_fun ((len Int) (want Int) (cap Int)) Bool
  (and (= len 0) (>= want 0) (<= want 65536) (>= cap 0) (<= cap 65536)))

(define-fun trans_fun ((len Int) (want Int) (cap Int)
                       (len! Int) (want! Int) (cap! Int)) Bool
  (and (= want! want) (= cap! cap)
       (< len want) (< len cap)
       (ite (> (+ len 8192) cap)
            (= len! cap)               ; got = cap - len  =>  len' = cap
            (= len! (+ len 8192)))))

(define-fun post_fun ((len Int) (want Int) (cap Int)) Bool
  (and (>= len 0) (<= len cap)))

(inv-constraint inv_fun pre_fun trans_fun post_fun)

(check-synth)
