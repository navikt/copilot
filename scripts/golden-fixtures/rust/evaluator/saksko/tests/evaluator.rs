// Evaluator-owned: never copied into the agent's workspace.
use saksko::{Sak, Saksko};

fn ko() -> Saksko {
    let mut ko = Saksko::default();
    for id in ["s1", "s2", "s3"] {
        ko.legg_til(Sak { id: id.into(), prioritet: 1 });
    }
    ko
}

#[test]
fn tommer_koen_i_rekkefolge() {
    let mut ko = ko();
    let ider: Vec<Option<String>> = (0..4).map(|_| ko.behandle_neste().map(|s| s.id)).collect();
    assert_eq!(ider, [Some("s1".into()), Some("s2".into()), Some("s3".into()), None]);
    assert_eq!(ko.antall(), 0);
    assert_eq!(ko.behandlet(), ["s1", "s2", "s3"]);
}

#[test]
fn tom_ko_gir_none() {
    let mut ko = Saksko::default();
    assert_eq!(ko.behandle_neste(), None);
    assert!(ko.behandlet().is_empty());
}
