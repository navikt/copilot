// Evaluator-owned: never copied into the agent's workspace.
use meldingsmottak::{Mottak, Utbetaling};

#[derive(Default)]
struct Bank {
    utbetalt: Vec<String>,
    feil_neste: bool,
}

impl Utbetaling for Bank {
    fn utbetal(&mut self, id: &str, _belop: u64) -> Result<(), String> {
        if std::mem::take(&mut self.feil_neste) {
            return Err("banken svarte ikke".into());
        }
        self.utbetalt.push(id.to_string());
        Ok(())
    }
}

#[test]
fn samme_id_utbetales_en_gang() {
    let mut m = Mottak::new(Bank::default());
    m.motta("e1", 100).unwrap();
    m.motta("e1", 100).unwrap();
    m.motta("e2", 50).unwrap();
    assert_eq!(m.utbetaling.utbetalt, ["e1", "e2"]);
}

// The kf1 trap: marking the id before the payment means a failed payment is
// skipped as a duplicate when the message is delivered again.
#[test]
fn feilet_utbetaling_utbetales_ved_ny_levering() {
    let mut m = Mottak::new(Bank { feil_neste: true, ..Default::default() });
    assert!(m.motta("e1", 100).is_err());
    assert!(!m.er_behandlet("e1"));
    m.motta("e1", 100).unwrap();
    assert_eq!(m.utbetaling.utbetalt, ["e1"]);
    assert!(m.er_behandlet("e1"));
}
