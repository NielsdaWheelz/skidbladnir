package sessionui

import tea "charm.land/bubbletea/v2"

func (m *model) pointerInput(message tea.MouseMsg, painted pointerHit) tea.Cmd {
	pressed := m.pressedPointer
	m.pressedPointer = pointerTarget{}
	mouse := message.Mouse()
	if mouse.Mod != 0 || m.busy || m.fold.loss != "" {
		return nil
	}
	if _, wheel := message.(tea.MouseWheelMsg); wheel {
		delta := 1
		if mouse.Button == tea.MouseWheelUp || mouse.Button == tea.MouseWheelLeft {
			delta = -1
		}
		switch mouse.Button {
		case tea.MouseWheelUp, tea.MouseWheelDown:
			switch m.fold.kind {
			case foldNone, foldOpen:
				m.move(delta)
			case foldBranch:
				if painted.preview == m.fold.branch && m.currentPointer(mouse.X, mouse.Y).preview == painted.preview {
					m.scrollBranch(delta)
				}
			}
		case tea.MouseWheelLeft, tea.MouseWheelRight:
			if m.fold.kind == foldNone || m.fold.kind == foldOpen {
				m.step(delta)
			}
		}
		return nil
	}
	if mouse.Button != tea.MouseLeft || (m.fold.kind != foldNone && m.fold.kind != foldOpen) {
		return nil
	}
	target := painted.target
	if target.kind == 0 {
		return nil
	}
	_, release := message.(tea.MouseReleaseMsg)
	if release && pressed != target {
		return nil
	}
	if m.currentPointer(mouse.X, mouse.Y).target != target {
		m.fail("listing changed. click again.")
		// An identical picture can keep the old renderer callback. Repaint the hit map too.
		return tea.ClearScreen
	}
	if !release {
		m.pressedPointer = target
		return nil
	}
	switch target.kind {
	case pointerAttach:
		m.navigateRow(m.rowIndex(target.ref))
		return m.attach(target.ref)
	case pointerDisclose:
		m.navigateRow(m.rowIndex(m.tree[target.parent].row.Session.Ref.Reference))
		m.setExpanded(target.parent, !m.expanded[target.parent])
	case pointerChoose:
		m.chooseView(target.view)
	}
	return nil
}
