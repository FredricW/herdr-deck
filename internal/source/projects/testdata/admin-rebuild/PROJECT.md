+++
name = "Admin rebuild"
goal = "Complete the Linear project 'Admin rebuild' (P-ABC-49) — at minimum land the new layouts for the admin pages (/admin/plans, /admin/users, /templates, /reports) in main. /checkout is out of scope for now. Design: https://www.figma.com/design/Fk3pQw8sLm2Nx7Rt5Vy9Zb/Design"
coordinator_profile = "claude"
thread_profile = "claude"
max_parallel_threads = 10
auto_resolve_days = 7
nudge = true
mute = false

[[repos]]
path = "/home/dev/src/webshop"
+++

# Instructions

- Stay away from the /checkout page for now (ABC-1235, ABC-1190, ABC-1193 and
  anything else under /checkout). The separate project `checkout-flow` owns
  that work; do not touch its routes, components or branch.

The settings above, between the `+++` lines, are changed from the projects
popup or by asking the coordinator.
