use clap::Parser;
use dashmap::DashMap;
use serde::{Deserialize, Serialize};
use std::sync::Arc;

#[derive(Parser, Debug)]
#[command(version, about, long_about = None)]
pub(crate) struct Args {
    #[arg(short, long, default_value = "8080")]
    pub(crate) port: u16,
}

pub(crate) struct Session {
    pub(crate) sender: String,
    pub(crate) receiver: Option<String>,
    pub(crate) offer: String,
}
pub(crate) type Sessions = Arc<DashMap<String, Session>>;

#[derive(Clone)]
pub(crate) struct AppState {
    pub(crate) sessions: Sessions,
    pub(crate) wordlist: Vec<String>,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "camelCase")]
pub(crate) enum SignalingMessage {
    #[serde(rename_all = "camelCase")]
    Offer {
        passphrase_length: u8,
        sdp: String,
    },
    Passphrase {
        passphrase: String,
    },
    Answer {
        sdp: String,
    },
    #[serde(rename = "ice-candidate")]
    IceCandidate {
        candidate: Option<String>,
    },
    #[serde(rename = "connection-request")]
    ConnectionRequest {
        passphrase: String,
    },
}
